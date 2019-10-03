package api

import (
	"strings"
	"time"
)

// DirectoriesResponse is the response for https://sia.tech/docs/#renter-dir-siapath-get
type DirectoriesResponse struct {
	Directories []DirectoryInfo `json:"directories"`
	Files       []FileInfo      `json:"files"`
}

// FilesResponse is the response for https://sia.tech/docs/#renter-files-get
type FilesResponse struct {
	Files []FileInfo `json:"files"`
}

// FileResponse is the response for https://sia.tech/docs/#renter-file-siapath-get
type FileResponse struct {
	File FileInfo `json:"file"`
}

// FileInfo is used in https://sia.tech/docs/#renter-files-get
type FileInfo struct {
	AccessTime       time.Time `json:"accesstime"`
	Available        bool      `json:"available"`
	ChangeTime       time.Time `json:"changetime"`
	CipherType       string    `json:"ciphertype"`
	CreateTime       time.Time `json:"createtime"`
	Expiration       uint64    `json:"expiration"`
	Filesize         uint64    `json:"filesize"`
	Health           float64   `json:"health"`
	LocalPath        string    `json:"localpath"`
	MaxHealth        float64   `json:"maxhealth"`
	MaxHealthPercent float64   `json:"maxhealthpercent"`
	ModTime          time.Time `json:"modtime"`
	NumStuckChunks   uint64    `json:"numstuckchunks"`
	OnDisk           bool      `json:"ondisk"`
	Recoverable      bool      `json:"recoverable"`
	Redundancy       float64   `json:"redundancy"`
	Renewing         bool      `json:"renewing"`
	SiaPath          string    `json:"siapath"`
	Stuck            bool      `json:"stuck"`
	StuckHealth      float64   `json:"stuckhealth"`
	UploadedBytes    uint64    `json:"uploadedbytes"`
	UploadProgress   float64   `json:"uploadprogress"`
}

// DirectoryInfo is used in https://sia.tech/docs/#renter-dir-siapath-get
type DirectoryInfo struct {
	AggregateHealth              float64   `json:"aggregatehealth"`
	AggregateLastHealthCheckTime time.Time `json:"aggregatelasthealthchecktime"`
	AggregateMaxHealth           float64   `json:"aggregatemaxhealth"`
	AggregateMaxHealthPercentage float64   `json:"aggregatemaxhealthpercentage"`
	AggregateMinRedundancy       float64   `json:"aggregateminredundancy"`
	AggregateMostRecentModTime   time.Time `json:"aggregatemostrecentmodtime"`
	AggregateNumFiles            uint64    `json:"aggregatenumfiles"`
	AggregateNumStuckChunks      uint64    `json:"aggregatenumstuckchunks"`
	AggregateNumSubDirs          uint64    `json:"aggregatenumsubdirs"`
	AggregateSize                uint64    `json:"aggregatesize"`
	AggregateStuckHealth         float64   `json:"aggregatestuckhealth"`

	Health              float64   `json:"health"`
	LastHealthCheckTime time.Time `json:"lasthealthchecktime"`
	MaxHealthPercentage float64   `json:"maxhealthpercentage"`
	MaxHealth           float64   `json:"maxhealth"`
	MinRedundancy       float64   `json:"minredundancy"`
	MostRecentModTime   time.Time `json:"mostrecentmodtime"`
	NumFiles            uint64    `json:"numfiles"`
	NumStuckChunks      uint64    `json:"numstuckchunks"`
	NumSubDirs          uint64    `json:"numsubdirs"`
	SiaPath             string    `json:"siapath"`
	Size                uint64    `json:"size"`
	StuckHealth         float64   `json:"stuckhealth"`
}

// Error contains an error message per https://sia.tech/docs/#error
type Error struct {
	Message    string `json:"message"`
	Status     string
	StatusCode int
}

// Error returns a string for the error and satisfies the error interface
func (e *Error) Error() string {
	var out []string
	if e.Message != "" {
		out = append(out, e.Message)
	}
	if e.Status != "" {
		out = append(out, e.Status)
	}
	if len(out) == 0 {
		return "Siad Error"
	}
	return strings.Join(out, ": ")
}
