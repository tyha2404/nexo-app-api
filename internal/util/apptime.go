package util

import (
	"sync/atomic"
	"time"
	_ "time/tzdata" // embed the tz database so the default zone always loads
)

// DefaultTimezone is the timezone of the app's users (Vietnam, UTC+7, no DST).
const DefaultTimezone = "Asia/Ho_Chi_Minh"

var appLocation atomic.Pointer[time.Location]

func init() {
	loc, err := time.LoadLocation(DefaultTimezone)
	if err != nil {
		loc = time.FixedZone("ICT", 7*60*60)
	}
	appLocation.Store(loc)
}

// SetAppTimezone sets the location used by AppLocation/Now. Call it once at
// startup with config.AppTimezone. An empty name keeps the default.
func SetAppTimezone(name string) error {
	if name == "" {
		return nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return err
	}
	appLocation.Store(loc)
	return nil
}

// AppLocation returns the app's business timezone (APP_TIMEZONE).
func AppLocation() *time.Location {
	return appLocation.Load()
}

// Now returns the current time in the app's timezone. Use it instead of
// time.Now() whenever "today" or "this month" matters: the server may run
// in UTC while users are in Vietnam.
func Now() time.Time {
	return time.Now().In(AppLocation())
}

// StartOfDay returns midnight of t's calendar day in the app timezone.
func StartOfDay(t time.Time) time.Time {
	t = t.In(AppLocation())
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, AppLocation())
}

// MonthRange returns [start, end) of the calendar month containing t, in the
// app timezone. end is the first instant of the next month.
func MonthRange(t time.Time) (start, end time.Time) {
	t = t.In(AppLocation())
	start = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, AppLocation())
	return start, start.AddDate(0, 1, 0)
}

// MonthRangeOf returns [start, end) for the given year/month in the app timezone.
func MonthRangeOf(year int, month time.Month) (start, end time.Time) {
	start = time.Date(year, month, 1, 0, 0, 0, 0, AppLocation())
	return start, start.AddDate(0, 1, 0)
}

// IsPastDate reports whether the calendar date of d (read in d's own location,
// i.e. the date the user picked, as stored in a DATE/TIMESTAMP column) is
// before today in the app timezone. A due date of today is NOT past.
func IsPastDate(d time.Time) bool {
	return DateString(d) < Today()
}

// DateString formats the calendar date of t in t's own location (YYYY-MM-DD).
func DateString(t time.Time) string {
	return t.Format("2006-01-02")
}

// Today returns today's date (YYYY-MM-DD) in the app timezone. Pass it to SQL
// as CAST(? AS date) to compare DATE/TIMESTAMP columns without timezone shifts.
func Today() string {
	return Now().Format("2006-01-02")
}
