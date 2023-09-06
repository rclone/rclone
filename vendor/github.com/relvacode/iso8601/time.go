package iso8601

import "time"

// The following is copied from the Go standard library to implement date range validation logic
// equivalent to the behaviour of Go's time.Parse.

func isLeap(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// daysInMonth is the number of days for non-leap years in each calendar month starting at 1
var daysInMonth = [...]int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

func daysIn(m time.Month, year int) int {
	if m == time.February && isLeap(year) {
		return 29
	}
	return daysInMonth[int(m)]
}
