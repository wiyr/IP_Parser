package qqwry

import (
	"encoding/binary"
	"errors"
	"io/ioutil"
	"net"
	"os"

	"github.com/wiyr/mahonia"
)

const (
	INDEX_LEN       = 7
	REDIRECT_MODE_1 = 0x01
	REDIRECT_MODE_2 = 0x02
)

type Result struct {
	Country string
	Area    string
}

type ipData struct {
	ipBelong []Result
	ipStart  []uint32
	ipEnd    []uint32
	ip2Num   []uint32
	data     []byte
	dataLen  uint32
	offset   uint32
}

var ips *ipData

func InitIpData(filePath string) error {
	ips = &ipData{}

	_, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filePath, os.O_RDONLY, 0400)
	defer file.Close()

	if err != nil {
		return err
	}

	ips.data, err = ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	ips.dataLen = uint32(len(ips.data))
	if ips.dataLen < 8 {
		return errors.New("QQwry.dat damaged(too short)")
	}
	header := ips.data[:8]
	start := binary.LittleEndian.Uint32(header[:4])
	end := binary.LittleEndian.Uint32(header[4:])
	if end < start || (end-start)%INDEX_LEN != 0 {
		return errors.New("QQwry.dat damaged(too short)")
	}

	n := int((end-start)/INDEX_LEN + 1)

	ips.ipBelong = make([]Result, n)
	ips.ip2Num = make([]uint32, n)
	for i, j := start, 0; i <= end; i += 7 {
		ips.ip2Num[j] = binary.LittleEndian.Uint32(ips.data[i : i+4])
		j += 1
	}

	ips.ipStart = make([]uint32, 256)
	ips.ipEnd = make([]uint32, 256)
	for i, j := 0, 0; i < n; i = j {
		for j = i + 1; j < n; j++ {
			if (ips.ip2Num[j] >> 24) != (ips.ip2Num[i] >> 24) {
				break
			}
		}
		ips.ipStart[ips.ip2Num[i]>>24] = uint32(i)
		ips.ipEnd[ips.ip2Num[i]>>24] = uint32(j - 1)
	}

	ips.offset = uint32(0)
	return nil
}

func SearchIpLocation(ip string) (Result, error) {
	res := Result{}

	ipAddress := net.ParseIP(ip)
	if ipAddress == nil {
		return res, errors.New("invalid ip")
	}

	index := binarySearch(binary.BigEndian.Uint32(ipAddress.To4()))

	return resolveIndex(index)
}

func resolveIndex(index uint32) (Result, error) {
	res := Result{}
	if ips.ipBelong[index] != res {
		return ips.ipBelong[index], nil
	}

	var country []byte
	var area []byte

	start := binary.LittleEndian.Uint32(ips.data[:4])
	start += index * INDEX_LEN
	offset := byte3ToUint32(ips.data[start+4 : start+7])
	countryOffset := offset + 4

	var err error
	var tempOffset uint32
	switch readMode(countryOffset) {
	case 0:
	case REDIRECT_MODE_1:
		if countryOffset, err = readUint24(); err != nil {
			return res, err
		}
		switch readMode(countryOffset) {
		case REDIRECT_MODE_2:
			if tempOffset, err = readUint24(); err != nil {
				return res, err
			}
			if country, err = readString(tempOffset); err != nil {
				return res, err
			}
			countryOffset += 4
		default:
			if country, err = readString(countryOffset); err != nil {
				return res, err
			}
			countryOffset += uint32(len(country)) + 1
		}
	case REDIRECT_MODE_2:
		if tempOffset, err = readUint24(); err != nil {
			return res, err
		}
		if country, err = readString(tempOffset); err != nil {
			return res, err
		}
		countryOffset += 4
	default:
		if country, err = readString(countryOffset); err != nil {
			return res, err
		}
		countryOffset += uint32(len(country)) + 1
	}
	if area, err = readArea(countryOffset); err != nil {
		return res, err
	}

	enc := mahonia.NewDecoder("gbk")
	res.Country = enc.ConvertString(string(country))
	res.Area = enc.ConvertString(string(area))

	ips.ipBelong[index] = res

	return res, nil
}

func binarySearch(ip uint32) uint32 {
	low := ips.ipStart[ip>>24]
	high := ips.ipEnd[ip>>24]

	//log.Println(low, high)

	mid := uint32(0)
	_ip := uint32(0)

	for low < high {
		mid = uint32((low + high + 1) / 2)
		_ip = ips.ip2Num[mid]

		if _ip < ip {
			low = mid
		} else if _ip > ip {
			high = mid - 1
		} else {
			return mid
		}
	}
	return low
}

func readFromIpData(num uint32, offset ...uint32) ([]byte, error) {
	if len(offset) > 0 {
		ips.offset = offset[0]
	}
	end := ips.offset + num
	if end > ips.dataLen {
		return []byte(""), errors.New("read ip form data out of index")
	}
	res := make([]byte, num)
	res = ips.data[ips.offset:end]
	ips.offset = end
	return res, nil
}

func readString(offset uint32) ([]byte, error) {
	ips.offset = offset
	var max_len_info uint32 = 500 // max length of info
	end := offset + max_len_info
	if end > ips.dataLen {
		end = ips.dataLen
	}
	for ; ips.offset < end; ips.offset++ {

		if ips.data[ips.offset] == 0 {
			break
		}
	}
	if ips.offset == end {
		return nil, errors.New("readString error, name too long")
	}
	return ips.data[offset:ips.offset], nil
}

func readArea(offset uint32) ([]byte, error) {
	switch readMode(offset) {
	case REDIRECT_MODE_1:
		fallthrough
	case REDIRECT_MODE_2:
		tempOffset, err := readUint24()
		if err != nil {
			return []byte(""), err
		}
		return readString(tempOffset)
	case 0:
		return []byte(""), nil
	default:
		return readString(offset)
	}
	return []byte(""), nil
}

func readMode(offset uint32) byte {
	res, _ := readFromIpData(1, offset)
	return res[0]
}

func readUint24() (uint32, error) {
	res, err := readFromIpData(3) // ..
	if err != nil {
		return uint32(0), err
	}
	return byte3ToUint32(res), nil
}

func byte3ToUint32(b []byte) uint32 {
	res := uint32(0)
	for i := 2; i >= 0; i-- {
		res <<= 8
		res += uint32(b[i])
	}
	return res
}

func uint32ToIp(value uint32) string {
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, value)
	ip := net.IP(buffer)
	return ip.String()
}
