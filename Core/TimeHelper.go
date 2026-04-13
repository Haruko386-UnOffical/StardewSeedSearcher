package Core

import "fmt"

type TimeHelper struct{}

const (
	DaysPerSeason  = 28
	SeasonsPerYear = 4
	DaysPerYear    = DaysPerSeason * SeasonsPerYear
)

// DateToAbsoluteDay 转换为绝对天数（从 Year1 Spring1 = 0 开始）
func DateToAbsoluteDay(year, season, day int) int {
	yearOffset := (year - 1) * DaysPerYear
	seasonOffset := season * DaysPerSeason
	dayOffset := day

	return yearOffset + seasonOffset + dayOffset
}

// AbsoluteDayToDate 从绝对天数还原为 (Year, Season, Day)
func AbsoluteDayToDate(absoluteDay int) (year, season, day int) {
	dayOfYear := absoluteDay % DaysPerYear
	if dayOfYear == 0 {
		dayOfYear = DaysPerYear
	}

	year = (absoluteDay-dayOfYear)/DaysPerYear + 1

	day = dayOfYear % DaysPerSeason
	if day == 0 {
		day = DaysPerSeason
	}

	season = (dayOfYear - day) / DaysPerSeason
	return year, season, day
}

// GetSeasonName 获取季节的中文名称
func GetSeasonName(season int) string {
	switch season {
	case 0:
		return "春"
	case 1:
		return "夏"
	case 2:
		return "秋"
	case 3:
		return "冬"
	default:
		return "未知"
	}
}

type GameDate struct {
	Year   int `json:"year"`
	Season int `json:"season"`
	Day    int `json:"day"`
}

func (g GameDate) ToAbsoluteDate() int {
	return DateToAbsoluteDay(g.Year, g.Season, g.Day)
}

func (g GameDate) SeasonName() string {
	return GetSeasonName(g.Season)
}

func (g GameDate) String() string {
	return fmt.Sprintf("%d年%s%d日", g.Year, g.SeasonName(), g.Day)
}
