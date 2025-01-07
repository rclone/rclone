// Package api defines types for interacting with the FileLu API
package api


// FolderListResponse represents the response from the folder/list API
type FolderListResponse struct {
    Status int    `json:"status"`
    Msg    string `json:"msg"`
    Result struct {
        Files []struct {
            Name      string `json:"name"`
            Size      int64  `json:"size"`
            Uploaded  string `json:"uploaded"`
            Thumbnail string `json:"thumbnail"`
            Link      string `json:"link"`
            FldID     int    `json:"fld_id"`      // Changed to int
            FileCode  string `json:"file_code"`
            Hash      string `json:"hash"`
        } `json:"files"`
        Folders []struct {
            Name      string `json:"name"`
            Code      string `json:"code"`
            FldID     int    `json:"fld_id"`      // Changed to int
            FldPublic int    `json:"fld_public"`
            Filedrop  int    `json:"filedrop"`
        } `json:"folders"`
    } `json:"result"`
}





type AccountInfoResponse struct {
    Status int    `json:"status"`
    Msg    string `json:"msg"`
    Result struct {
        PremiumExpire string `json:"premium_expire"`
        Email         string `json:"email"`
        UType         string `json:"utype"`
        Storage       string `json:"storage"`
        StorageUsed   string `json:"storage_used"`
    } `json:"result"`
}

type FolderDeleteResponse struct {
    Status     int    `json:"status"`
    Msg        string `json:"msg"`
    Result     string `json:"result"`
    ServerTime string `json:"server_time"`
}

type FolderListResponse_File = struct {
    Name      string `json:"name"`
    Size      int64  `json:"size"`
    Uploaded  string `json:"uploaded"`
    Thumbnail string `json:"thumbnail"`
    Link      string `json:"link"`
    FldID     int    `json:"fld_id"`
    FileCode  string `json:"file_code"`
    Hash      string `json:"hash"`
}


type DeleteResponse struct {
    Status int    `json:"status"`
    Msg    string `json:"msg"`
}
