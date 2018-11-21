package rsencoding

import (
	"errors"
	"strconv"
)

const (
	base10Len          = 20
	base32Len          = 13
	initialCodewordLen = 17
)

var initialCodeword = [initialCodewordLen]int{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
var gexp = [32]int{1, 2, 4, 8, 16, 5, 10, 20, 13, 26, 17, 7, 14, 28, 29, 31,
	27, 19, 3, 6, 12, 24, 21, 15, 30, 25, 23, 11, 22, 9, 18, 1}
var codewordMap = [initialCodewordLen]int{3, 2, 1, 0, 7, 6, 5, 4, 13, 14, 15, 16, 12, 8, 9, 10, 11}
var alphabet = [32]byte{'2', '3', '4', '5', '6', '7', '8', '9', 'A', 'B', 'C', 'D', 'E', 'F',
	'G', 'H', 'J', 'K', 'L', 'M', 'N', 'P', 'Q', 'R', 'S', 'T', 'U', 'V',
	'W', 'X', 'Y', 'Z'}
var golg = [32]int{0, 0, 1, 18, 2, 5, 19, 11, 3, 29, 6, 27, 20, 8, 12, 23, 4, 10, 30, 17,
	7, 22, 28, 26, 21, 25, 9, 16, 13, 14, 24, 15}

func gmult(x int, y int) int {
	if x == 0 || y == 0 {
		return 0
	}

	i := (golg[x] + golg[y]) % 31

	return gexp[i]
}

// Encode account id to burst address format 8KLL-PBYV-6DBC-AM942
func Encode(accountID uint64) string {
	plainString := strconv.FormatUint(accountID, 10)
	length := len(plainString)

	var plainString10 [base10Len]int
	for i := range plainString {
		plainString10[i] = int(plainString[i]) - 48
	}

	var codeword [initialCodewordLen]int
	codewordLength := 0
	for {
		newLength := 0
		digit32 := 0

		for i := 0; i < length; i++ {
			digit32 = digit32*10 + plainString10[i]

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
	for i := base32Len - 1; i >= 0; i-- {
		fb := codeword[i] ^ p[3]
		p[3] = p[2] ^ gmult(30, fb)
		p[2] = p[1] ^ gmult(6, fb)
		p[1] = p[0] ^ gmult(9, fb)
		p[0] = gmult(17, fb)
	}

	copy(codeword[base32Len:base32Len+len(p)], p[:])

	var buff [20]byte
	for i, j := 0, 0; i < initialCodewordLen; i, j = i+1, j+1 {
		codewordIndex := codewordMap[i]
		alphabetIndex := codeword[codewordIndex]

		buff[j] = alphabet[alphabetIndex]

		if (i&3) == 3 && i < 13 {
			j++
			buff[j] = '-'
		}
	}

	return string(buff[:])
}

// Decode burst address to account id
func Decode(address string) (uint64, error) {
	codeword := initialCodeword

	codewordLength := 0
	for i := 0; i < len(address); i++ {
		positionInAlphabet := -1
		for j, char := range alphabet {
			if char == address[i] {
				positionInAlphabet = j
				break
			}
		}

		if positionInAlphabet < 0 {
			continue
		}

		if codewordLength > 16 {
			return 0, errors.New("codeword too long for cyperstring")
		}

		codewordIndex := codewordMap[codewordLength]
		codeword[codewordIndex] = positionInAlphabet
		codewordLength++
	}

	if codewordLength == initialCodewordLen && !isCodewordValid(&codeword) || codewordLength != initialCodewordLen {
		return 0, errors.New("codeword invalid")
	}

	length := base32Len
	var cyperString32 [base32Len]int
	for i := 0; i < length; i++ {
		cyperString32[i] = codeword[length-i-1]
	}

	var plainParts [20]byte
	var plainPartsLen int
	for plainPartsLen = 0; ; plainPartsLen++ {
		newLength := 0
		digit10 := 0

		for j := 0; j < length; j++ {
			digit10 = digit10*32 + cyperString32[j]

			if digit10 >= 10 {
				cyperString32[newLength] = digit10 / 10
				digit10 %= 10
				newLength++
			} else if newLength > 0 {
				cyperString32[newLength] = 0
				newLength++
			}
		}
		length = newLength
		plainParts[-plainPartsLen+19] = []byte(strconv.Itoa(digit10))[0]

		if length <= 0 {
			break
		}
	}

	res, err := strconv.ParseUint(string(plainParts[19-plainPartsLen:]), 10, 64)
	if err != nil {
		return 0, err
	}
	return res, nil
}

func isCodewordValid(codeword *[initialCodewordLen]int) bool {
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
