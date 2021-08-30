//go:generate stringer -output=type_string.go -type=AddrType,CfgLoadType,LoadStatusType
package snmp

import (
	"errors"
	"fmt"
	"net"
	"time"

	// currently not imported by gosnmp
	_ "crypto/aes"
	_ "crypto/des"
	_ "crypto/md5"
	_ "crypto/sha1"
	_ "crypto/sha256"
	_ "crypto/sha512"

	"github.com/gosnmp/gosnmp"
	"github.com/korylprince/ruckus-config-backup/tftp"
)

const (
	snmpPassword           = ".1.3.6.1.4.1.1991.1.1.2.1.15.0"
	snmpTFTPServerAddrType = ".1.3.6.1.4.1.1991.1.1.2.1.65.0"
	snmpTFTPServerAddr     = ".1.3.6.1.4.1.1991.1.1.2.1.66.0"
	snmpTFTPCfgName        = ".1.3.6.1.4.1.1991.1.1.2.1.8.0"
	snmpTFTPLoad           = ".1.3.6.1.4.1.1991.1.1.2.1.9.0"
)

type AddrType int

const AddrIPv4 AddrType = 1

type CfgLoadType int

const CfgLoadRunningConfigDownload CfgLoadType = 22

type LoadStatusType int

const (
	LoadStatusNormal                   LoadStatusType = 1
	LoadStatusFlashPrepareReadFailure  LoadStatusType = 2
	LoadStatusFlashReadError           LoadStatusType = 3
	LoadStatusFlashPrepareWriteFailure LoadStatusType = 4
	LoadStatusFlashWriteError          LoadStatusType = 5
	LoadStatusTftpTimeoutError         LoadStatusType = 6
	LoadStatusTftpOutOfBufferSpace     LoadStatusType = 7
	LoadStatusTftpBusy                 LoadStatusType = 8
	LoadStatusTftpRemoteOtherErrors    LoadStatusType = 9
	LoadStatusTftpRemoteNoFile         LoadStatusType = 10
	LoadStatusTftpRemoteBadAccess      LoadStatusType = 11
	LoadStatusTftpRemoteDiskFull       LoadStatusType = 12
	LoadStatusTftpRemoteBadOperation   LoadStatusType = 13
	LoadStatusTftpRemoteBadID          LoadStatusType = 14
	LoadStatusTftpRemoteFileExists     LoadStatusType = 15
	LoadStatusTftpRemoteNoUser         LoadStatusType = 16
	LoadStatusOperationError           LoadStatusType = 17
	LoadStatusLoading                  LoadStatusType = 18
)

type Config struct {
	Port         uint16
	Username     string
	AuthPassword string
	PrivPassword string
	AuthProtocol gosnmp.SnmpV3AuthProtocol
	PrivProtocol gosnmp.SnmpV3PrivProtocol

	Timeout        time.Duration
	Retries        int
	MaxOIDs        int
	MaxRepetitions uint32
}

var DefaultConfig = &Config{
	Port:           161,
	AuthProtocol:   gosnmp.SHA,
	PrivProtocol:   gosnmp.AES,
	Timeout:        time.Second * 5,
	Retries:        2,
	MaxOIDs:        gosnmp.MaxOids,
	MaxRepetitions: 25, // gosnmp.defaultMaxRepetitions / 2 to prevent timeout issues
}

func (c *Config) New(host string) *gosnmp.GoSNMP {
	return &gosnmp.GoSNMP{
		Target:         host,
		Port:           c.Port,
		Version:        gosnmp.Version3,
		Timeout:        c.Timeout,
		Retries:        c.Retries,
		MaxOids:        c.MaxOIDs,
		MaxRepetitions: c.MaxRepetitions,
		MsgFlags:       gosnmp.AuthPriv,
		SecurityModel:  gosnmp.UserSecurityModel,
		SecurityParameters: &gosnmp.UsmSecurityParameters{
			AuthenticationProtocol:   c.AuthProtocol,
			UserName:                 c.Username,
			AuthenticationPassphrase: c.AuthPassword,
			PrivacyProtocol:          c.PrivProtocol,
			PrivacyPassphrase:        c.PrivPassword,
		},
	}
}

func (c *Config) DownloadConfig(host string, svr *tftp.Server) error {
	snmp := c.New(host)
	if err := snmp.Connect(); err != nil {
		return fmt.Errorf("could not open SNMP connection to host %s: %w", host, err)
	}
	defer snmp.Conn.Close()

	addr := (snmp.Conn.LocalAddr()).(*net.UDPAddr).IP
	svr.Handle(addr)

	ip := []byte(addr)
	if len(ip) == 16 {
		ip = ip[12:]
	}

	res, err := snmp.Set([]gosnmp.SnmpPDU{
		{Name: snmpPassword, Type: gosnmp.OctetString, Value: c.AuthPassword},
		{Name: snmpTFTPServerAddrType, Type: gosnmp.Integer, Value: int(AddrIPv4)},
		{Name: snmpTFTPServerAddr, Type: gosnmp.OctetString, Value: ip},
		{Name: snmpTFTPCfgName, Type: gosnmp.OctetString, Value: host + ".conf"},
		{Name: snmpTFTPLoad, Type: gosnmp.Integer, Value: int(CfgLoadRunningConfigDownload)},
	})
	if err != nil {
		return fmt.Errorf("could not issue SNMP set: %w", err)
	}
	if res.Error != gosnmp.NoError {
		return fmt.Errorf("snmp set failed: %v", res.Error)
	}

loop:
	for {
		time.Sleep(time.Second)
		pkt, err := snmp.Get([]string{snmpTFTPLoad})
		if err != nil {
			return fmt.Errorf("could not check status: %w", err)
		}

		for _, p := range pkt.Variables {
			if p.Name == snmpTFTPLoad {
				switch lt, ok := (p.Value).(int); {
				case !ok:
					return errors.New("could not check status: invalid return type")
				case LoadStatusType(lt) == LoadStatusNormal:
					return nil
				case LoadStatusType(lt) == LoadStatusLoading:
					continue loop
				default:
					return fmt.Errorf("download failed: %v", LoadStatusType(lt))
				}
			}
		}
		return errors.New("could not check status: load status not returned")
	}
}
