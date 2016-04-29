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
	data      []byte
	data_len  uint32
	file_path string
	p_file    *os.File
}

type QQwry struct {
	p_ip_data *ipData
	offset    uint32
}

func ReadIpData(file_path string) (*ipData, error) {
	res := &ipData{file_path: file_path}

	_, err := os.Stat(file_path)
	if err != nil {
		return res, err
	}

	file, err := os.OpenFile(file_path, os.O_RDONLY, 0400)
	defer file.Close()

	if err != nil {
		return res, err
	}
	res.p_file = file

	res.data, err = ioutil.ReadAll(file)
	if err != nil {
		return res, err
	}

	res.data_len = uint32(len(res.data))

	return res, nil
}

func NewQQwry(p *ipData) *QQwry {
	return &QQwry{p, 0}
}

func (this *QQwry) SearchIpLocation(ip string) (Result, error) {
	result := Result{}

	if this.p_ip_data == nil {
		return result, errors.New("from QQwry p_ip_data is nil")
	}

	ip_address := net.ParseIP(ip)
	if ip_address == nil {
		return result, errors.New("invalid ip")
	}

	offset := this.binarySearch(binary.BigEndian.Uint32(ip_address.To4()))

	if offset <= 0 {
		return result, errors.New("ip not found")
	}

	var country []byte
	var area []byte

	country_offset := offset + 4

	switch this.readMode(country_offset) {
	case 0:
	case REDIRECT_MODE_1:
		country_offset = this.readUint24()
		switch this.readMode(country_offset) {
		case REDIRECT_MODE_2:
			temp_offset := this.readUint24()
			country = this.readString(temp_offset)
			country_offset += 4
		default:
			country = this.readString(country_offset)
			country_offset += uint32(len(country)) + 1
		}
	case REDIRECT_MODE_2:
		temp_offset := this.readUint24()
		country = this.readString(temp_offset)
		country_offset += 4
	default:
		country = this.readString(country_offset)
		country_offset += uint32(len(country)) + 1
	}
	area = this.readArea(country_offset)

	enc := mahonia.NewDecoder("gbk")
	result.Country = enc.ConvertString(string(country))
	result.Area = enc.ConvertString(string(area))
	return result, nil
}

func (this *QQwry) binarySearch(ip uint32) uint32 {
	buffer := this.readFromIpData(8, 0)
	start := binary.LittleEndian.Uint32(buffer[:4])
	end := binary.LittleEndian.Uint32(buffer[4:])

	total := (end - start) / INDEX_LEN
	low, high := uint32(0), total

	temp_index := make([]byte, INDEX_LEN)
	mid := uint32(0)
	_ip := uint32(0)

	//log.Println(uint32ToIp(ip))
	for low < high {
		mid = (low + high + 1) / 2
		temp_index, _ip = this.binaryCheck(start + mid*INDEX_LEN)

		if _ip < ip {
			low = mid
		} else if _ip > ip {
			high = mid - 1
		} else {
			return byte3ToUint32(temp_index[4:])
		}
	}
	temp_index, _ = this.binaryCheck(start + low*INDEX_LEN)
	return byte3ToUint32(temp_index[4:])
}

func (this *QQwry) binaryCheck(value uint32) ([]byte, uint32) {
	buffer := this.readFromIpData(INDEX_LEN, value)
	res := binary.LittleEndian.Uint32(buffer[:4])
	return buffer, res
}

func (this *QQwry) readFromIpData(num uint32, offset ...uint32) []byte {
	if len(offset) > 0 {
		this.offset = offset[0]
	}
	end := this.offset + num
	if end > this.p_ip_data.data_len {
		return nil
	}
	res := make([]byte, num)
	res = this.p_ip_data.data[this.offset:end]
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
		temp_offset := this.readUint24()
		return this.readString(temp_offset)
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
