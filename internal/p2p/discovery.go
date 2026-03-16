package p2p

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

const (
	ServiceName = "_clipboard._tcp"
)

type Discovery struct {
	deviceID   string
	deviceName string
	port       int
	logger     *slog.Logger
	onPeer     func(deviceID, address string)
}

func NewDiscovery(deviceID, deviceName string, port int, logger *slog.Logger, onPeer func(string, string)) *Discovery {
	return &Discovery{
		deviceID:   deviceID,
		deviceName: deviceName,
		port:       port,
		logger:     logger,
		onPeer:     onPeer,
	}
}

// Advertise announces the device on the local network
func (d *Discovery) Advertise(ctx context.Context) error {
	info := []string{
		fmt.Sprintf("device_id=%s", d.deviceID),
		fmt.Sprintf("device_name=%s", d.deviceName),
	}

	ips, err := d.lanIPs()
	if err != nil {
		return fmt.Errorf("failed to get LAN IPs: %w", err)
	}

	service, err := mdns.NewMDNSService(
		d.deviceID,
		ServiceName,
		"",
		"",
		d.port,
		ips,
		info,
	)
	if err != nil {
		return fmt.Errorf("failed to create mDNS service: %w", err)
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return fmt.Errorf("failed to start mDNS server: %w", err)
	}

	go func() {
		<-ctx.Done()
		err := server.Shutdown()
		if err != nil {
			d.logger.Error("Failed to shutdown mDNS server", "error", err)
		}
	}()

	return nil
}

func (d *Discovery) Discover(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			d.scan()
		}
	}
}

func (d *Discovery) scan() {
	entriesCh := make(chan *mdns.ServiceEntry, 10)

	go func() {
		for entry := range entriesCh {
			// Ignore self
			if strings.Contains(entry.Name, d.deviceID) {
				continue
			}

			deviceID := extractField(entry.InfoFields, "device_id")
			if deviceID == "" {
				continue
			}

			addr := fmt.Sprintf("%s:%d", entry.AddrV4, entry.Port)

			if d.onPeer != nil {
				d.onPeer(deviceID, addr)
			}
		}
	}()

	if err := mdns.Query(&mdns.QueryParam{
		Service: ServiceName,
		Timeout: 3 * time.Second,
		Entries: entriesCh,
	}); err != nil {
		d.logger.Error("mDNS query failed", "error", err)
	}
	close(entriesCh)

}

// lanIPs returns the LAN-facing IPv4 address by opening a no-op UDP connection
// to a public DNS server and reading back the local address the OS chose.
// This avoids relying on mdns's default interface enumeration, which may
// advertise unreachable addresses like loopback, Docker bridge, or VPN interfaces.
func (d *Discovery) lanIPs() ([]net.IP, error) {
	conn, err := net.Dial("udp4", "8.8.8.8:53")
	if err != nil {
		return nil, fmt.Errorf("failed to determine outbound IP: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			d.logger.Error("Failed to close UDP connection", "error", err)
		}
	}()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return []net.IP{localAddr.IP}, nil
}

func extractField(fields []string, key string) string {
	prefix := key + "="
	for _, field := range fields {
		if strings.HasPrefix(field, prefix) {
			return strings.TrimPrefix(field, prefix)
		}
	}
	return ""
}
