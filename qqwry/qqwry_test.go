package qqwry

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"testing"
)

func Benchmark(b *testing.B) {
	rand.Seed(42)
	pIpData, err := InitIpData("/usr/local/share/QQWry.Dat")
	if err != nil {
		log.Println(err)
		return
	}

	p_qqwry, err := NewQQwry(pIpData)
	if err != nil {
		return
	}

	for i := 0; i < b.N; i++ {
		test_case := 1
		for j := 0; j < test_case; j++ {
			ip_s := getIpString()
			_, err := p_qqwry.SearchIpLocation(ip_s) //string(os.Args[1]))
			if err != nil {
				fmt.Println(err, ip_s)
				continue
			}
			//fmt.Printf("Search: %s\nCountry: %s\nArea: %s\n", ip_s, res.Country, res.Area)
		}
	}
}

func getIpString() string {
	b := make([]byte, 4)
	for i := 0; i < 4; i++ {
		b[i] = byte(rand.Intn(256))
	}
	ip := net.IP(b)
	return ip.String()
}
