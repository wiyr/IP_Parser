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
	ipStart  []uint32
	ipEnd    []uint32
	ip2Num   []uint32
	data     []byte
	dataLen  uint32
	filePath string
	pFile    *os.File
}

type QQwry struct {
	pIpData *ipData
	offset  uint32
}

func InitIpData(filePath string) (*ipData, error) {
	res := &ipData{filePath: filePath}

	_, err := os.Stat(filePath)
	if err != nil {
		return res, err
	}

	file, err := os.OpenFile(filePath, os.O_RDONLY, 0400)
	defer file.Close()

	if err != nil {
		return res, err
	}
	res.pFile = file

	res.data, err = ioutil.ReadAll(file)
	if err != nil {
		return res, err
	}

	res.dataLen = uint32(len(res.data))
	if res.dataLen < 8 {
		return res, errors.New("QQwry.dat damaged(too short)")
	}
	header := res.data[:8]
	start := binary.LittleEndian.Uint32(header[:4])
	end := binary.LittleEndian.Uint32(header[4:])
	if end < start {
		return res, errors.New("QQwry.dat damaged(too short)")
	}

	n := int((end-start)/INDEX_LEN + 1)

	res.ip2Num = make([]uint32, n)
	for i, j := start, 0; i <= end; i += 7 {
		res.ip2Num[j] = binary.LittleEndian.Uint32(res.data[i : i+4])
		j += 1
	}

	res.ipStart = make([]uint32, 256)
	res.ipEnd = make([]uint32, 256)
	for i, j := 0, 0; i < n; i = j {
		for j = i + 1; j < n; j++ {
			if (res.ip2Num[j] >> 24) != (res.ip2Num[i] >> 24) {
				break
			}
		}
		res.ipStart[res.ip2Num[i]>>24] = uint32(i)
		res.ipEnd[res.ip2Num[i]>>24] = uint32(j - 1)
	}

	return res, nil
}

func NewQQwry(p *ipData) *QQwry {
	return &QQwry{p, 0}
}

func (this *QQwry) SearchIpLocation(ip string) (Result, error) {
	result := Result{}

	if this.pIpData == nil {
		return result, errors.New("from QQwry pIpData is nil")
	}

	ipAddress := net.ParseIP(ip)
	if ipAddress == nil {
		return result, errors.New("invalid ip")
	}

	offset := this.binarySearch(binary.BigEndian.Uint32(ipAddress.To4()))

	var country []byte
	var area []byte

	countryOffset := offset + 4

	switch this.readMode(countryOffset) {
	case 0:
	case REDIRECT_MODE_1:
		countryOffset = this.readUint24()
		switch this.readMode(countryOffset) {
		case REDIRECT_MODE_2:
			tempOffset := this.readUint24()
			country = this.readString(tempOffset)
			countryOffset += 4
		default:
			country = this.readString(countryOffset)
			countryOffset += uint32(len(country)) + 1
		}
	case REDIRECT_MODE_2:
		tempOffset := this.readUint24()
		country = this.readString(tempOffset)
		countryOffset += 4
	default:
		country = this.readString(countryOffset)
		countryOffset += uint32(len(country)) + 1
	}
	area = this.readArea(countryOffset)

	enc := mahonia.NewDecoder("gbk")
	result.Country = enc.ConvertString(string(country))
	result.Area = enc.ConvertString(string(area))
	return result, nil
}

func (this *QQwry) binarySearch(ip uint32) uint32 {
	start := binary.LittleEndian.Uint32(this.pIpData.data[:4])

	low := this.pIpData.ipStart[ip>>24]
	high := this.pIpData.ipEnd[ip>>24]

	//log.Println(low, high)

	mid := uint32(0)
	_ip := uint32(0)

	for low < high {
		mid = uint32((low + high + 1) / 2)
		_ip = this.pIpData.ip2Num[mid]

		if _ip < ip {
			low = mid
		} else if _ip > ip {
			high = mid - 1
		} else {
			ipIndex := start + mid*INDEX_LEN + 4
			return byte3ToUint32(this.pIpData.data[ipIndex : ipIndex+3])
		}
	}
	ipIndex := start + low*INDEX_LEN + 4
	return byte3ToUint32(this.pIpData.data[ipIndex : ipIndex+3])
}

func (this *QQwry) readFromIpData(num uint32, offset ...uint32) []byte {
	if len(offset) > 0 {
		this.offset = offset[0]
	}
	end := this.offset + num
	if end > this.pIpData.dataLen {
		return nil
	}
	res := make([]byte, num)
	res = this.pIpData.data[this.offset:end]
	this.offset = end
	return res
}

func (this *QQwry) readString(offset uint32) []byte {
	this.offset = offset
	var max_len_info = 50
	res := make([]byte, 0, max_len_info)
	buffer := make([]byte, 1)
	for i := 0; i < max_len_info; i++ {
		buffer = this.readFromIpData(1)
		if buffer[0] == 0 {
			break
		}
		res = append(res, buffer[0])
	}
	return res
}

func (this *QQwry) readArea(offset uint32) []byte {
	switch this.readMode(offset) {
	case REDIRECT_MODE_1:
		fallthrough
	case REDIRECT_MODE_2:
		tempOffset := this.readUint24()
		return this.readString(tempOffset)
	case 0:
		return []byte("")
	default:
		return this.readString(offset)
	}
	return []byte("")
}

func (this *QQwry) readMode(offset uint32) byte {
	res := this.readFromIpData(1, offset)
	return res[0]
}

func (this *QQwry) readUint24() uint32 {
	res := this.readFromIpData(3) // ..
	return byte3ToUint32(res)
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
