// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package compare implements term operation helpers.
package compare

import (
	"strconv"
	"strings"
)

// GetNumbers returns numbers parsed from left and right params, if any of string to number conversion fails returns false as the 3rd arg.
func GetNumbers(left, right string) (int64, int64, bool) {
	numLeft, ok := parseValue(left)
	if !ok {
		return 0, 0, false
	}

	numRight, ok := parseValue(right)
	if !ok {
		return 0, 0, false
	}

	return numLeft, numRight, true
}

func parseValue(value string) (int64, bool) {
	value = strings.TrimSpace(value)

	splitPoint := len(value)

	for i, c := range value {
		if c >= '0' && c <= '9' || c == '-' {
			continue
		}

		splitPoint = i

		break
	}

	digits, units := value[:splitPoint], value[splitPoint:]

	if len(digits) == 0 {
		return 0, false
	}

	res, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, false
	}

	multiplier, ok := getMultiplier(units)
	if !ok {
		return 0, false
	}

	return res * multiplier, true
}

func getMultiplier(value string) (int64, bool) {
	value = strings.TrimSpace(strings.ToLower(value))

	if len(value) == 0 {
		return 1, true
	}

	if len(value) > 1 {
		switch strings.ToLower(value[:2]) {
		case "pi":
			return 1 << 50, true
		case "ti":
			return 1 << 40, true
		case "gi":
			return 1 << 30, true
		case "mi":
			return 1 << 20, true
		case "ki":
			return 1 << 10, true
		}
	}

	switch strings.ToLower(value[:1]) {
	case "p":
		return 1e15, true
	case "t":
		return 1e12, true
	case "g":
		return 1e9, true
	case "m":
		return 1e6, true
	case "k":
		return 1e3, true
	}

	return 0, false
}
