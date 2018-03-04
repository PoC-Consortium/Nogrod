// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package rsencoding

import (
	. "logger"
	"strconv"

	"go.uber.org/zap"
)

var initialCodeword [17]int
var gexp [32]int
var codewordMap [17]int
var alphabet [32]byte
var base32Length int
var base10Length int

func init() {
	initialCodeword = [...]int{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	gexp = [...]int{
		1, 2, 4, 8, 16, 5, 10, 20, 13, 26, 17, 7, 14, 28, 29, 31, 27, 19,
		3, 6, 12, 24, 21, 15, 30, 25, 23, 11, 22, 9, 18, 1}

	codewordMap = [...]int{
		3, 2, 1, 0, 7, 6, 5, 4, 13, 14, 15, 16, 12, 8, 9, 10, 11}

	alphabet = [...]byte{
		'2', '3', '4', '5', '6', '7', '8', '9', 'A', 'B', 'C', 'D', 'E', 'F',
		'G', 'H', 'J', 'K', 'L', 'M', 'N', 'P', 'Q', 'R', 'S', 'T', 'U', 'V',
		'W', 'X', 'Y', 'Z',
	}

	base32Length = 13
	base10Length = 20
}

func gmult(x int, y int) int {
	if x == 0 || y == 0 {
		return 0
	}

	golg := []int{
		0, 0, 1, 18, 2, 5, 19, 11, 3, 29, 6, 27, 20, 8, 12, 23, 4, 10, 30, 17,
		7, 22, 28, 26, 21, 25, 9, 16, 13, 14, 24, 15}

	i := (golg[x] + golg[y]) % 31

	return gexp[i]
}

func Encode(plain uint64) string {
	plainString := strconv.FormatUint(plain, 10)
	length := len(plainString)

	var plainString10 = make([]int, base10Length)
	for i, _ := range plainString {
		plainString10[i] = int(plainString[i]) - 48
	}

	codeword := make([]int, len(initialCodeword))
	codewordLength := 0
	for {
		newLength := 0
		digit32 := 0

		for i := 0; i < length; i++ {
			digit32 = digit32*10 + int(plainString10[i])

			if digit32 >= 32 {
				plainString10[newLength] = digit32 >> 5
				digit32 &= 31
				newLength++
			} else if newLength > 0 {
				plainString10[newLength] = 0
				newLength++
			}
		}
		length = newLength
		codeword[codewordLength] = digit32
		codewordLength++

		if length <= 0 {
			break
		}
	}

	p := []int{0, 0, 0, 0}
	for i := base32Length - 1; i >= 0; i-- {
		fb := codeword[i] ^ p[3]
		p[3] = p[2] ^ gmult(30, fb)
		p[2] = p[1] ^ gmult(6, fb)
		p[1] = p[0] ^ gmult(9, fb)
		p[0] = gmult(17, fb)
	}

	copy(codeword[base32Length:base32Length+len(p)], p[:])

	var buff []byte
	for i := 0; i < 17; i++ {
		codewordIndex := codewordMap[i]
		alphabetIndex := codeword[codewordIndex]

		buff = append(buff, alphabet[alphabetIndex])

		if (i&3) == 3 && i < 13 {
			buff = append(buff, '-')
		}
	}

	return string(buff)
}

func Decode(cyperString string) uint64 {
	codeword := initialCodeword

	codewordLength := 0
	for i := 0; i < len(cyperString); i++ {
		positionInAlphabet := -1
		for j, char := range alphabet {
			if char == cyperString[i] {
				positionInAlphabet = j
				break
			}
		}

		if positionInAlphabet < 0 {
			continue
		}

		if codewordLength > 16 {
			Logger.Error("codeword to long for cyperstring", zap.String("cyperString", cyperString))
			return 0
		}

		codewordIndex := codewordMap[codewordLength]
		codeword[codewordIndex] = positionInAlphabet
		codewordLength++
	}

	if codewordLength == 17 && !isCodewordValid(codeword) || codewordLength != 17 {
		Logger.Error("codeword invalid", zap.Ints("codeword", codeword[:]))
		return 0
	}

	length := base32Length
	cyperString32 := make([]int, length)
	for i := 0; i < length; i++ {
		cyperString32[i] = codeword[length-i-1]
	}

	plainParts := ""
	for {
		newLength := 0
		digit10 := 0

		for i := 0; i < length; i++ {
			digit10 = digit10*32 + cyperString32[i]

			if digit10 >= 10 {
				cyperString32[newLength] = int(digit10 / 10)
				digit10 %= 10
				newLength++
			} else if newLength > 0 {
				cyperString32[newLength] = 0
				newLength++
			}
		}
		length = newLength
		plainParts += strconv.Itoa(digit10)

		if length <= 0 {
			break
		}
	}

	runes := []rune(plainParts)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	res, err := strconv.ParseUint(string(runes), 10, 64)
	if err != nil {
		Logger.Error("failed to read uint64 from runes", zap.String("runes", string(runes)))
		return 0
	}
	return res
}

func isCodewordValid(codeword [17]int) bool {
	sum := 0
	for i := 1; i < 5; i++ {
		t := 0

		for j := 0; j < 31; j++ {
			if j > 12 && j < 27 {
				continue
			}

			pos := j
			if j > 26 {
				pos -= 14
			}

			t ^= gmult(codeword[pos], gexp[(i*j)%31])
		}

		sum |= t
	}

	return sum == 0
}
