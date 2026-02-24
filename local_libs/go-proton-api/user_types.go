package proton

type User struct {
	ID          string
	Name        string
	DisplayName string
	Email       string
	Keys        Keys

	UsedSpace int64
	MaxSpace  int64
	MaxUpload int64

	Credit   int64
	Currency string
}

type DeleteUserReq struct {
	Reason   string
	Feedback string
	Email    string
}
