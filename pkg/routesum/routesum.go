// Package routesum summarizes a list of IPs and networks to its shortest form.
package routesum

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/pkg/errors"
	"github.com/stamp/routesum/pkg/routesum/bitslice"
	"github.com/stamp/routesum/pkg/routesum/rstrie"
)

// RouteSum has methods supporting route summarization of networks and hosts
type RouteSum struct {
	ipv4, ipv6 *rstrie.RSTrie
}

// NewRouteSum returns an initialized RouteSum object
func NewRouteSum() *RouteSum {
	rs := new(RouteSum)
	rs.ipv4 = rstrie.NewRSTrie()
	rs.ipv6 = rstrie.NewRSTrie()

	return rs
}

// InsertFromString adds either a string-formatted network or IP to the summary
func (rs *RouteSum) InsertFromString(s string) error {
	var ip netip.Addr
	var ipBits bitslice.BitSlice
	var err error

	if strings.Contains(s, "/") {
		ipPrefix, err := netip.ParsePrefix(s)
		if err != nil {
			return fmt.Errorf("parse network: %w", err)
		}
		if !ipPrefix.IsValid() {
			return errors.Errorf("%s is not valid CIDR", s)
		}

		ip = ipPrefix.Addr()
		ipBits, err = ipBitsForIPPrefix(ipPrefix)
		if err != nil {
			return err
		}
	} else {
		ip, err = netip.ParseAddr(s)
		if err != nil {
			return fmt.Errorf("parse IP: %w", err)
		}
		if !ip.IsValid() {
			return errors.Errorf("%s is not a valid IP", s)
		}

		ipBits, err = ipBitsForIP(ip)
		if err != nil {
			return err
		}
	}

	if ip.Is4() {
		rs.ipv4.InsertRoute(ipBits)
	} else {
		rs.ipv6.InsertRoute(ipBits)
	}

	return nil
}

func ipBitsForIPPrefix(ipPrefix netip.Prefix) (bitslice.BitSlice, error) {
	ipBytes, err := ipPrefix.Addr().MarshalBinary()
	if err != nil {
		return nil, errors.Wrapf(err, "express %s as bytes", ipPrefix.Addr().String())
	}

	ipBits, err := bitslice.NewFromBytes(ipBytes)
	if err != nil {
		return nil, fmt.Errorf("express %s as bits: %w", ipPrefix.Addr().String(), err)
	}

	return ipBits[:ipPrefix.Bits()], nil
}

func ipBitsForIP(ip netip.Addr) (bitslice.BitSlice, error) {
	ipBytes, err := ip.MarshalBinary()
	if err != nil {
		return nil, errors.Wrapf(err, "express %s as bytes", ip.String())
	}

	ipBits, err := bitslice.NewFromBytes(ipBytes)
	if err != nil {
		return nil, fmt.Errorf("express %s as bits: %w", ip.String(), err)
	}

	return ipBits, nil
}

// SummaryStrings returns a summary of all received routes as a string slice.
func (rs *RouteSum) SummaryStrings() []string {
	strs := []string{}

	ipv4BitSlices := rs.ipv4.Contents()
	for _, bits := range ipv4BitSlices {
		ip := ipv4FromBits(bits)

		if len(bits) == 8*4 {
			strs = append(strs, ip.String())
		} else {
			ipPrefix := netip.PrefixFrom(ip, len(bits))
			strs = append(strs, ipPrefix.String())
		}
	}

	ipv6BitSlices := rs.ipv6.Contents()
	for _, bits := range ipv6BitSlices {
		ip := ipv6FromBits(bits)

		if len(bits) == 8*16 {
			strs = append(strs, ip.String())
		} else {
			ipPrefix := netip.PrefixFrom(ip, len(bits))
			strs = append(strs, ipPrefix.String())
		}
	}

	return strs
}

// Summary returns a summary of all received routes as a slice of IPs and a slice of networks.
func (rs *RouteSum) Summary() ([]netip.Addr, []netip.Prefix) {
	ips := []netip.Addr{}
	nets := []netip.Prefix{}

	ipv4BitSlices := rs.ipv4.Contents()
	for _, bits := range ipv4BitSlices {
		ip := ipv4FromBits(bits)

		if len(bits) == 8*4 {
			ips = append(ips, ip)
		} else {
			ipPrefix := netip.PrefixFrom(ip, len(bits))
			nets = append(nets, ipPrefix)
		}
	}

	ipv6BitSlices := rs.ipv6.Contents()
	for _, bits := range ipv6BitSlices {
		ip := ipv6FromBits(bits)

		if len(bits) == 8*16 {
			ips = append(ips, ip)
		} else {
			ipPrefix := netip.PrefixFrom(ip, len(bits))
			nets = append(nets, ipPrefix)
		}
	}

	return ips, nets
}

func ipv4FromBits(bits bitslice.BitSlice) netip.Addr {
	bytes := bits.ToBytes(4)
	byteArray := [4]byte{}
	copy(byteArray[:], bytes[0:4])
	return netip.AddrFrom4(byteArray)
}

func ipv6FromBits(bits bitslice.BitSlice) netip.Addr {
	bytes := bits.ToBytes(16)
	byteArray := [16]byte{}
	copy(byteArray[:], bytes[0:16])
	return netip.AddrFrom16(byteArray)
}
