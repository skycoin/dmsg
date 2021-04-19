package proxyproto

import (
	"bufio"
	"bytes"
	"net"
	"strconv"
	"strings"
)

const (
	crlf      = "\r\n"
	separator = " "
)

func initVersion1() *Header {
	header := new(Header)
	header.Version = 1
	// Command doesn't exist in v1
	header.Command = PROXY
	return header
}

func parseVersion1(reader *bufio.Reader) (*Header, error) {
	// Make sure we have a v1 header
	line, err := reader.ReadString('\n')
	if !strings.HasSuffix(line, crlf) {
		return nil, ErrCantReadProtocolVersionAndCommand
	}
	tokens := strings.Split(line[:len(line)-2], separator)
	if len(tokens) < 6 {
		return nil, ErrCantReadProtocolVersionAndCommand
	}

	header := initVersion1()

	// Read address family and protocol
	switch tokens[1] {
	case "TCP4":
		header.TransportProtocol = TCPv4
	case "TCP6":
		header.TransportProtocol = TCPv6
	default:
		header.TransportProtocol = UNSPEC
	}

	// Read addresses and ports
	sourceIP, err := parseV1IPAddress(header.TransportProtocol, tokens[2])
	if err != nil {
		return nil, err
	}
	destIP, err := parseV1IPAddress(header.TransportProtocol, tokens[3])
	if err != nil {
		return nil, err
	}
	sourcePort, err := parseV1PortNumber(tokens[4])
	if err != nil {
		return nil, err
	}
	destPort, err := parseV1PortNumber(tokens[5])
	if err != nil {
		return nil, err
	}
	header.SourceAddr = &net.TCPAddr{
		IP:   sourceIP,
		Port: sourcePort,
	}
	header.DestinationAddr = &net.TCPAddr{
		IP:   destIP,
		Port: destPort,
	}

	return header, nil
}

func (header *Header) formatVersion1() ([]byte, error) {
	// As of version 1, only "TCP4" ( \x54 \x43 \x50 \x34 ) for TCP over IPv4,
	// and "TCP6" ( \x54 \x43 \x50 \x36 ) for TCP over IPv6 are allowed.
	var proto string
	switch header.TransportProtocol {
	case TCPv4:
		proto = "TCP4"
	case TCPv6:
		proto = "TCP6"
	default:
		// Unknown connection (short form)
		return []byte("PROXY UNKNOWN\r\n"), nil
	}

	sourceAddr, sourceOK := header.SourceAddr.(*net.TCPAddr)
	destAddr, destOK := header.DestinationAddr.(*net.TCPAddr)
	if !sourceOK || !destOK {
		return nil, ErrInvalidAddress
	}

	sourceIP, destIP := sourceAddr.IP, destAddr.IP
	switch header.TransportProtocol {
	case TCPv4:
		sourceIP = sourceIP.To4()
		destIP = destIP.To4()
	case TCPv6:
		sourceIP = sourceIP.To16()
		destIP = destIP.To16()
	}
	if sourceIP == nil || destIP == nil {
		return nil, ErrInvalidAddress
	}

	buf := bytes.NewBuffer(make([]byte, 0, 108))
	buf.Write(SIGV1)
	buf.WriteString(separator)
	buf.WriteString(proto)
	buf.WriteString(separator)
	buf.WriteString(sourceIP.String())
	buf.WriteString(separator)
	buf.WriteString(destIP.String())
	buf.WriteString(separator)
	buf.WriteString(strconv.Itoa(sourceAddr.Port))
	buf.WriteString(separator)
	buf.WriteString(strconv.Itoa(destAddr.Port))
	buf.WriteString(crlf)

	return buf.Bytes(), nil
}

func parseV1PortNumber(portStr string) (int, error) {
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 0 || port > 65535 {
		return 0, ErrInvalidPortNumber
	}
	return port, nil
}

func parseV1IPAddress(protocol AddressFamilyAndProtocol, addrStr string) (addr net.IP, err error) {
	addr = net.ParseIP(addrStr)
	tryV4 := addr.To4()
	if (protocol == TCPv4 && tryV4 == nil) || (protocol == TCPv6 && tryV4 != nil) {
		err = ErrInvalidAddress
	}
	return
}
