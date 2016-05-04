package qq

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"testing"
	"time"
)

func Benchmark(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	err := InitIpData("/usr/local/share/QQWry.Dat")
	if err != nil {
		log.Println(err)
		return
	}

	for i := 0; i < b.N; i++ {
		test_case := 1
		for j := 0; j < test_case; j++ {
			ip_s := getIpString()
			res, err := SearchIpLocation(ip_s) //string(os.Args[1]))
			if err != nil {
				fmt.Println(err, ip_s)
				continue
			}
			fmt.Printf("Search: %s\nCountry: %s\nArea: %s\n", ip_s, res.Country, res.Area)
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
