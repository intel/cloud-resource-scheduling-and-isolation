/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package policy

import (
	"encoding/json"
	"fmt"
	"io"
	"k8s.io/klog/v2"
	"math/bits"
	"os"
	"path/filepath"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sort"
	"strconv"
	"strings"
)

type catSchemaType string

const (
	CacheIdAll                         = "all"
	catSchemaTypeUnified catSchemaType = "unified"
	catSchemaTypeCode    catSchemaType = "code"
	catSchemaTypeData    catSchemaType = "data"
	schemata                           = "schemata"
)

type CatConfig map[string]CacheIdCatConfig

type catConfig CatConfig

type CacheIdCatConfig struct {
	Unified CacheProportion
	Code    CacheProportion
	Data    CacheProportion
}

type cacheIdCatConfig CacheIdCatConfig

type CacheProportion string

type CatSchema struct {
	Lvl   CacheLevel
	Alloc CatSchemaRaw
}

type CacheLevel string

type CatSchemaRaw map[uint64]CatAllocation

type CatAllocation struct {
	Unified cacheAllocation
	Code    cacheAllocation `json:",omitempty"`
	Data    cacheAllocation `json:",omitempty"`
}

type cacheAllocation interface {
	Transform(uint64, bitmask) (bitmask, error)
}

type catPercentageAllocation uint64

func (a catPercentageAllocation) Transform(minBits uint64, baseMask bitmask) (bitmask, error) {
	return catPercentageRangeAllocation{highRange: uint64(a)}.Transform(minBits, baseMask)
}

// catPctRangeAllocation represents a percentage range of the available bitmask
type catPercentageRangeAllocation struct {
	lowRange  uint64
	highRange uint64
}

// Transform function of the cacheAllocation interface
func (a catPercentageRangeAllocation) Transform(minBits uint64, baseMask bitmask) (bitmask, error) {
	if err := verifyCatBaseMask(baseMask, minBits); err != nil {
		return 0, err
	}

	baseMaskNumBits := uint64(baseMask.msbOne()) - uint64(baseMask.lsbOne()) + 1
	l, h := a.lowRange, a.highRange
	if l == 0 {
		l = 1
	}
	if l > h || h > 100 || l > 100 {
		return 0, fmt.Errorf("invalid percentage range in %v", a)
	}

	lsb := (l - 1) * baseMaskNumBits / 100
	msb := (h - 1) * baseMaskNumBits / 100
	numBits := msb - lsb + 1
	if numBits < minBits {
		gap := minBits - numBits

		if gap > lsb {
			gap -= lsb
			lsb = 0
		} else {
			lsb -= gap
			gap = 0
		}
		msbAvailable := baseMaskNumBits - msb - 1
		if gap <= msbAvailable {
			msb += gap
		} else {
			return 0, fmt.Errorf("not enough bits available for cache bitmask, %v applied on basemask %#x", a, baseMask)
		}
	}

	value := ((1 << (msb - lsb + 1)) - 1) << (lsb + uint64(baseMask.lsbOne()))

	return bitmask(value), nil
}

type catAbsoluteAllocation bitmask

// Transform function of the cacheAllocation interface
func (a catAbsoluteAllocation) Transform(minBits uint64, baseMask bitmask) (bitmask, error) {
	if err := verifyCatBaseMask(baseMask, minBits); err != nil {
		return 0, err
	}

	shiftWidth := baseMask.lsbOne()

	bmask := bitmask(a) << shiftWidth

	if bmask|baseMask != baseMask {
		return 0, fmt.Errorf("bitmask %#x (%#x << %d) does not fit basemask %#x", bmask, a, shiftWidth, baseMask)
	}

	return bmask, nil
}

func verifyCatBaseMask(baseMask bitmask, minBits uint64) error {
	if baseMask == 0 {
		return fmt.Errorf("empty basemask not allowed")
	}

	baseMaskWidth := baseMask.msbOne() - baseMask.lsbOne() + 1
	if bits.OnesCount64(uint64(baseMask)) != baseMaskWidth {
		return fmt.Errorf("invalid basemask %#x: more than one block of bits set", baseMask)
	}
	if uint64(bits.OnesCount64(uint64(baseMask))) < minBits {
		return fmt.Errorf("invalid basemask %#x: fewer than %d bits set", baseMask, minBits)
	}

	return nil
}

func (c CatConfig) toSchema(lvl CacheLevel) (CatSchema, error) {
	if c == nil {
		return CatSchema{Lvl: lvl}, nil
	}

	allocations := newCatSchema(lvl)
	minCbmBits := resctrlL3Info.info.minCbmBits()

	d, ok := c[CacheIdAll]
	if !ok {
		d = CacheIdCatConfig{Unified: "100%"}
	}
	defaultVal, err := d.transform(minCbmBits)
	if err != nil {
		return allocations, err
	}

	// Pre-fill with defaults
	for _, i := range resctrlL3Info.info.cacheIds {
		allocations.Alloc[i] = defaultVal
	}

	for key, val := range c {
		if key == CacheIdAll {
			continue
		}

		ids, err := listStringToArray(key)
		if err != nil {
			return allocations, err
		}

		schemaVal, err := val.transform(minCbmBits)
		if err != nil {
			return allocations, err
		}

		for _, id := range ids {
			if _, ok := allocations.Alloc[uint64(id)]; ok {
				allocations.Alloc[uint64(id)] = schemaVal
			}
		}
	}

	return allocations, nil
}

func newCatSchema(typ CacheLevel) CatSchema {
	return CatSchema{
		Lvl:   typ,
		Alloc: make(map[uint64]CatAllocation),
	}
}

func (c *CacheIdCatConfig) transform(minBits uint64) (CatAllocation, error) {
	allocation := CatAllocation{}
	var err error

	allocation.Unified, err = c.Unified.transform(minBits)
	if err != nil {
		return allocation, err
	}
	if allocation.Unified == nil {
		return allocation, fmt.Errorf("'unified' not specified in cache schema %s", *c)
	}

	allocation.Code, err = c.Code.transform(minBits)
	if err != nil {
		return allocation, err
	}
	allocation.Data, err = c.Data.transform(minBits)
	if err != nil {
		return allocation, err
	}
	if allocation.Code == nil && allocation.Data != nil {
		return allocation, fmt.Errorf("'data' specified but missing 'code' from cache schema %s", *c)
	}
	if allocation.Code != nil && allocation.Data == nil {
		return allocation, fmt.Errorf("'code' specified but missing 'data' from cache schema %s", *c)
	}

	return allocation, nil
}

func (c *CacheIdCatConfig) UnmarshalJSON(data []byte) error {
	raw := new(interface{})
	var err error
	err = json.Unmarshal(data, raw)
	if err != nil {
		return err
	}
	conf := CacheIdCatConfig{}
	switch v := (*raw).(type) {
	case string:
		conf.Unified = CacheProportion(v)
	default:
		helper := cacheIdCatConfig{}
		err = json.Unmarshal(data, &helper)
		if err != nil {
			return err
		}
		conf.Data = helper.Data
		conf.Code = helper.Code
		conf.Unified = helper.Unified
	}
	*c = conf
	return nil
}

func (c *CatConfig) UnmarshalJSON(data []byte) error {
	raw := new(interface{})

	err := json.Unmarshal(data, raw)
	if err != nil {
		return err
	}

	conf := CatConfig{}
	switch v := (*raw).(type) {
	case string:
		conf[CacheIdAll] = CacheIdCatConfig{Unified: CacheProportion(v)}
	default:
		helper := catConfig{}
		if err := json.Unmarshal(data, &helper); err != nil {
			return err
		}
		for k, v := range helper {
			conf[k] = v
		}
	}
	*c = conf
	return nil
}

func (c CacheProportion) transform(minBits uint64) (cacheAllocation, error) {
	if c == "" {
		return nil, nil
	}

	if c[len(c)-1] != '%' {
		// Absolute allocation
		var value uint64
		var err error
		if strings.HasPrefix(string(c), "0x") {
			value, err = strconv.ParseUint(string(c[2:]), 16, 64)
			if err != nil {
				return nil, err
			}
		} else {
			tmp, err := listStringToBitmask(string(c))
			value = uint64(tmp)
			if err != nil {
				return nil, err
			}
		}
		numOnes := bits.OnesCount64(value)
		if numOnes != bits.Len64(value)-bits.TrailingZeros64(value) {
			return nil, fmt.Errorf("invalid cache bitmask %q: more than one continuous block of ones", c)
		}
		if uint64(numOnes) < minBits {
			return nil, fmt.Errorf("invalid cache bitmask %q: number of bits less than %d", c, minBits)
		}

		return catAbsoluteAllocation(value), nil
	}

	// Percentages of the max number of bits
	split := strings.SplitN(string(c)[0:len(c)-1], "-", 2)
	var allocation cacheAllocation

	if len(split) == 1 {
		pct, err := strconv.ParseUint(split[0], 10, 7)
		if err != nil {
			return allocation, err
		}
		if pct > 100 {
			return allocation, fmt.Errorf("invalid percentage value %q", c)
		}
		allocation = catPercentageAllocation(pct)
	} else if len(split) == 2 {
		lowPct, err := strconv.ParseUint(split[0], 10, 7)
		if err != nil {
			return allocation, err
		}
		highPct, err := strconv.ParseUint(split[1], 10, 7)
		if err != nil {
			return allocation, err
		}
		if lowPct > highPct || lowPct > 100 || highPct > 100 {
			return allocation, fmt.Errorf("invalid percentage range %q", c)
		}
		allocation = catPercentageRangeAllocation{lowRange: lowPct, highRange: highPct}
	}

	return allocation, nil
}

// listStringToArray parses a string containing a human-readable list of numbers
// into an integer array
func listStringToArray(str string) ([]int, error) {
	a := []int{}

	if len(str) == 0 {
		return a, nil
	}

	ranges := strings.Split(str, ",")
	for _, ran := range ranges {
		split := strings.SplitN(ran, "-", 2)

		num, err := strconv.ParseInt(split[0], 10, 8)
		if err != nil {
			return a, fmt.Errorf("invalid integer %q: %v", str, err)
		}

		if len(split) == 1 {
			a = append(a, int(num))
		} else {
			endNum, err := strconv.ParseInt(split[1], 10, 8)
			if err != nil {
				return a, fmt.Errorf("invalid integer in range %q: %v", str, err)
			}
			if endNum <= num {
				return a, fmt.Errorf("invalid integer range %q in %q", ran, str)
			}
			for i := num; i <= endNum; i++ {
				a = append(a, int(i))
			}
		}
	}
	sort.Ints(a)
	return a, nil
}

// listStringToBitmask parses a string containing a human-readable list of bit
// numbers into a bitmask
func listStringToBitmask(str string) (bitmask, error) {
	b := bitmask(0)

	if len(str) == 0 {
		return b, nil
	}

	ranges := strings.Split(str, ",")
	for _, ran := range ranges {
		split := strings.SplitN(ran, "-", 2)

		beginNum, err := strconv.ParseUint(split[0], 10, 6)
		if err != nil {
			return b, fmt.Errorf("invalid bitmask %q: %v", str, err)
		}
		if len(split) == 2 {
			endNum, err := strconv.ParseUint(split[1], 10, 6)
			if err != nil {
				return b, fmt.Errorf("invalid bitmask %q: %v", str, err)
			}
			if endNum <= beginNum {
				return b, fmt.Errorf("invalid range %q in bitmask %q", ran, str)
			}
			b |= (1<<(endNum-beginNum+1) - 1) << beginNum
		} else if len(split) == 1 {
			b |= 1 << beginNum
		}
	}
	return b, nil
}

func getL3SchemaStr(catSchema CatSchema) (string, error) {
	schemata := ""
	switch {
	case resctrlL3Info.info.unified.Supported():
		schema, err := catSchema.toStr(catSchemaTypeUnified)
		if err != nil {
			return "", err
		}
		schemata += schema
	case resctrlL3Info.info.data.Supported() || resctrlL3Info.info.code.Supported():
		schema, err := catSchema.toStr(catSchemaTypeCode)
		if err != nil {
			return "", err
		}
		schemata += schema

		schema, err = catSchema.toStr(catSchemaTypeData)
		if err != nil {
			return "", err
		}
		schemata += schema
	default:
		if catSchema.Alloc != nil {
			return "", fmt.Errorf("l3 cache allocation for specified in configuration but not supported by system")
		}
	}
	return schemata, nil
}

func (s CatSchema) toStr(typ catSchemaType) (string, error) {
	schema := string(s.Lvl) + typ.toResctrlStr() + ":"
	sep := ""

	// Get a sorted slice of cache ids for deterministic output
	ids := append([]uint64{}, resctrlL3Info.info.cacheIds...)
	minCbmBits := resctrlL3Info.info.minCbmBits()
	for _, id := range ids {
		// Default to 100%
		bmask := resctrlL3Info.info.cbmMask()
		if s.Alloc != nil {
			var err error

			masks := s.Alloc[id]
			overlayMask := masks.getEffective(typ)

			bmask, err = overlayMask.Transform(minCbmBits, bmask)
			if err != nil {
				return "", err
			}
		}
		schema += fmt.Sprintf("%s%d=%x", sep, id, bmask)
		sep = ";"
	}

	return schema, nil
}

func (t catSchemaType) toResctrlStr() string {
	if t == catSchemaTypeUnified {
		return ""
	}
	return strings.ToUpper(string(t))
}

func (a CatAllocation) getEffective(typ catSchemaType) cacheAllocation {
	switch typ {
	case catSchemaTypeCode:
		if a.Code != nil {
			return a.Code
		}
	case catSchemaTypeData:
		if a.Data != nil {
			return a.Data
		}
	}
	// Use Unified as the default/fallback for Code and Data
	return a.Unified
}

func writeInSchemaFile(resource string, l3cache string, schemapath string) error {
	file, err := os.OpenFile(schemapath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, resource) {
			lines[i] = l3cache
		}
	}
	newContent := strings.Join(lines, "\n")
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Write([]byte(newContent)); err != nil {
		return err
	}
	return nil
}

func getL3Schema(l3 string) (string, error) {
	// convert "0xfff" to "L3:0=fff;1=fff"
	conf := CatConfig{}
	conf[CacheIdAll] = CacheIdCatConfig{Unified: CacheProportion(l3)}
	catSchema, err := conf.toSchema("L3")
	if err != nil {
		return "", err
	}
	l3cache, err := getL3SchemaStr(catSchema)
	if err != nil {
		return "", err
	}
	return l3cache, nil
}

func modifyRDTClassL3schema(className string, l3 string) error {
	l3Schema, err := getL3Schema(l3)
	if err != nil {
		return err
	}
	schemapath := filepath.Join(resctrlL3Info.resctrlPath, className, schemata)
	klog.V(utils.DBG).Info(schemapath, ": ", l3Schema)
	return writeInSchemaFile("L3", l3Schema, schemapath)
}

func checkBitmask(bitmask string) error {
	var value uint64
	var err error
	if strings.HasPrefix(string(bitmask), "0x") {
		value, err = strconv.ParseUint(string(bitmask[2:]), 16, 64)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("invalid cache bitmask %q: do not start with 0x", bitmask)
	}

	// Sanity check of absolute allocation: bitmask must (only) contain one
	// contiguous block of ones wide enough
	numOnes := bits.OnesCount64(value)
	if numOnes != bits.Len64(value)-bits.TrailingZeros64(value) {
		return fmt.Errorf("invalid cache bitmask %q: more than one continuous block of ones", bitmask)
	}
	minBits := resctrlL3Info.info.minCbmBits()
	if uint64(numOnes) < minBits {
		return fmt.Errorf("invalid cache bitmask %q: number of bits less than %d", bitmask, minBits)
	}
	return nil
}
