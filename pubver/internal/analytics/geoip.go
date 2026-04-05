package analytics

import (
	"net"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

type MaxMindGeoResolver struct {
	reader *geoip2.Reader
}

func NewMaxMindGeoResolver(path string) (*MaxMindGeoResolver, error) {
	reader, err := geoip2.Open(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	return &MaxMindGeoResolver{reader: reader}, nil
}

func (r *MaxMindGeoResolver) Resolve(ip string) (country, city string) {
	if r == nil || r.reader == nil {
		return "", ""
	}
	parsedIP := net.ParseIP(strings.TrimSpace(ip))
	if parsedIP == nil {
		return "", ""
	}

	record, err := r.reader.City(parsedIP)
	if err != nil {
		return "", ""
	}

	if record.Country.IsoCode != "" {
		country = record.Country.IsoCode
	} else if len(record.Country.Names) > 0 {
		country = record.Country.Names["en"]
	}
	if len(record.City.Names) > 0 {
		city = record.City.Names["en"]
	}

	return country, city
}

func (r *MaxMindGeoResolver) Close() error {
	if r == nil || r.reader == nil {
		return nil
	}
	return r.reader.Close()
}
