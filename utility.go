package utility

import (
	"fmt"
	"strconv"
	"time"
)

const Interval30Days = 2592000

func IsSameDay(t1, t2 int64) bool {
	y1, m1, d1 := time.Unix(t1, 0).Date()
	y2, m2, d2 := time.Unix(t2, 0).Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func CompareDay(t1, t2 int64) int {
	y1, m1, d1 := time.Unix(t1, 0).Date()
	y2, m2, d2 := time.Unix(t2, 0).Date()

	// 比较年份
	if y1 < y2 {
		return -1
	} else if y1 > y2 {
		return 1
	}

	// 年份相同，比较月份
	if m1 < m2 {
		return -1
	} else if m1 > m2 {
		return 1
	}

	// 年份、月份都相同，比较日期
	if d1 < d2 {
		return -1
	} else if d1 > d2 {
		return 1
	}

	// 年、月、日全相同
	return 0
}
func IsSameWeek(t1, t2 int64) bool {
	y1, w1 := time.Unix(t1, 0).ISOWeek()
	y2, w2 := time.Unix(t2, 0).ISOWeek()
	return y1 == y2 && w1 == w2
}

func IsSameMonth(t1, t2 int64) bool {
	y1, m1, _ := time.Unix(t1, 0).Date()
	y2, m2, _ := time.Unix(t2, 0).Date()
	return y1 == y2 && m1 == m2
}

func IsMonday(t1 int64) bool {
	t := time.Unix(t1, 0)
	d := t.Weekday()
	return d == time.Monday
}

func IsExceed2AM(t1 int64) bool {
	t := time.Unix(t1, 0)
	h := t.Hour()
	return h >= 2
}

func IsExceedSpecifyTime(t1 int64, hour, minute int) bool {
	t := time.Unix(t1, 0)
	currentHour := t.Hour()
	currentMinute := t.Minute()

	// 先比较小时
	if currentHour > hour {
		return true
	}

	// 小时相等时比较分钟
	if currentHour == hour && currentMinute >= minute {
		return true
	}

	return false
}

func IsInterval30Days(t1, t2 int64) bool {
	if t1 > t2 {
		t2, t1 = t1, t2
	}
	return t2-t1 >= Interval30Days
}

// 获取时间t当天的0点0时0分0秒的unix_time
func GetMidnightUnix(t time.Time) int64 {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Unix()
}

func GetLastOrTodayFiveAMUnix() int64 {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(loc)

	var targetDay time.Time
	if now.Hour() < 5 {
		targetDay = now.AddDate(0, 0, -1)
	} else {
		targetDay = now
	}

	fiveAM := time.Date(targetDay.Year(), targetDay.Month(), targetDay.Day(), 5, 0, 0, 0, loc)
	return fiveAM.Unix()
}

func GetValueFromString(srcData string, outData any) error {
	if 0 == len(srcData) {
		return nil
	}

	switch outData.(type) {
	case *int8:
		value, err := strconv.ParseInt(srcData, 10, 8)
		*outData.(*int8) = int8(value)
		return err
	case *int16:
		value, err := strconv.ParseInt(srcData, 10, 16)
		*outData.(*int16) = int16(value)
		return err
	case *int:
		value, err := strconv.ParseInt(srcData, 10, 32)
		*outData.(*int) = int(value)
		return err
	case *int32:
		value, err := strconv.ParseInt(srcData, 10, 32)
		*outData.(*int32) = int32(value)
		return err
	case *int64:
		var err error
		*outData.(*int64), err = strconv.ParseInt(srcData, 10, 64)
		return err
	case *uint8:
		value, err := strconv.ParseUint(srcData, 10, 8)
		*outData.(*uint8) = uint8(value)
		return err
	case *uint16:
		value, err := strconv.ParseUint(srcData, 10, 16)
		*outData.(*uint16) = uint16(value)
		return err
	case *uint32:
		value, err := strconv.ParseUint(srcData, 10, 32)
		*outData.(*uint32) = uint32(value)
		return err
	case *uint64:
		var err error
		*outData.(*uint64), err = strconv.ParseUint(srcData, 10, 64)
		return err
	case *float32:
		value, err := strconv.ParseFloat(srcData, 32)
		*outData.(*float32) = float32(value)
		return err
	case *float64:
		var err error
		*outData.(*float64), err = strconv.ParseFloat(srcData, 64)
		return err
	default:
		return fmt.Errorf("invalid type of outData:%T", outData)
	}
}

func Min[T int | int32 | int64 | float32 | float64](x, y T) T {
	if x < y {
		return x
	} else {
		return y
	}
}

func Abs[T int | int32 | int64 | float32 | float64](n T) T {
	if n < 0 {
		return -n
	}
	return n
}

func CalcAgeByIdNumber(IdNumber string) (int, error) {
	if len(IdNumber) != 18 {
		return 0, fmt.Errorf("invalid ID card length")
	}

	birthday, err := time.Parse("20060102", IdNumber[6:14]) // 20060102 是Go的日期格式
	if err != nil {
		return 0, err
	}

	now := time.Now()
	age := now.Year() - birthday.Year()
	if now.Month() < birthday.Month() || (now.Month() == birthday.Month() && now.Day() < birthday.Day()) {
		age-- // 如果生日还没到，年龄减一
	}

	return age, nil
}

// 判断字符串数组是否包含
func NotContainsAll(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return false
		}
	}
	return true
}

func NotContains(arr []string, s string) bool {
	// 计算需要检查的范围
	start := len(arr) - 5
	if start < 0 {
		start = 0
	}

	// 从后向前检查最后 5 个元素
	for i := len(arr) - 1; i >= start; i-- {
		if arr[i] == s {
			return false
		}
	}

	return true
}
