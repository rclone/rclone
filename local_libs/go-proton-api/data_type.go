package proton

type SendStatsReq struct {
	MeasurementGroup string
	Event            string
	Values           map[string]any
	Dimensions       map[string]any
}

type SendStatsMultiReq struct {
	EventInfo []SendStatsReq
}
