package gphotos_mobile

import (
	"time"
)

// buildMediaFieldMask builds the field mask for media item fields.
// This corresponds to the deeply nested protobuf structure in gpmc's get_library_state
// that tells the server which fields to return for each media item.
func buildMediaFieldMask() *ProtoBuilder {
	// field 1 = media item field mask (which fields to return)
	f1 := NewProtoBuilder()
	f1.AddEmptyMessage(1)
	f1.AddEmptyMessage(3)
	f1.AddEmptyMessage(4)

	// field 5 = camera info fields
	f5 := NewProtoBuilder()
	f5.AddEmptyMessage(1)
	f5.AddEmptyMessage(2)
	f5.AddEmptyMessage(3)
	f5.AddEmptyMessage(4)
	f5.AddEmptyMessage(5)
	f5.AddEmptyMessage(7)
	f1.AddMessage(5, f5)

	f1.AddEmptyMessage(6)

	f7 := NewProtoBuilder()
	f7.AddEmptyMessage(2)
	f1.AddMessage(7, f7)

	f1.AddEmptyMessage(15)
	f1.AddEmptyMessage(16)
	f1.AddEmptyMessage(17)
	f1.AddEmptyMessage(19)
	f1.AddEmptyMessage(20)

	f21 := NewProtoBuilder()
	f21_5 := NewProtoBuilder()
	f21_5.AddEmptyMessage(3)
	f21.AddMessage(5, f21_5)
	f21.AddEmptyMessage(6)
	f1.AddMessage(21, f21)

	f1.AddEmptyMessage(25)

	f30 := NewProtoBuilder()
	f30.AddEmptyMessage(2)
	f1.AddMessage(30, f30)

	f1.AddEmptyMessage(31)
	f1.AddEmptyMessage(32)

	f33 := NewProtoBuilder()
	f33.AddEmptyMessage(1)
	f1.AddMessage(33, f33)

	f1.AddEmptyMessage(34)
	f1.AddEmptyMessage(36)
	f1.AddEmptyMessage(37)
	f1.AddEmptyMessage(38)
	f1.AddEmptyMessage(39)
	f1.AddEmptyMessage(40)
	f1.AddEmptyMessage(41)

	return f1
}

// buildMediaTypeFieldMask builds the field mask for media type info (photo/video)
func buildMediaTypeFieldMask() *ProtoBuilder {
	f5 := NewProtoBuilder()

	// Photo info (field 2)
	f5_2 := NewProtoBuilder()
	f5_2_2 := NewProtoBuilder()
	f5_2_2_3 := NewProtoBuilder()
	f5_2_2_3.AddEmptyMessage(2)
	f5_2_2.AddMessage(3, f5_2_2_3)
	f5_2_2_4 := NewProtoBuilder()
	f5_2_2_4.AddEmptyMessage(2)
	f5_2_2.AddMessage(4, f5_2_2_4)
	f5_2.AddMessage(2, f5_2_2)

	f5_2_4 := NewProtoBuilder()
	f5_2_4_2 := NewProtoBuilder()
	f5_2_4_2.AddVarint(2, 1)
	f5_2_4.AddMessage(2, f5_2_4_2)
	f5_2.AddMessage(4, f5_2_4)

	f5_2_5 := NewProtoBuilder()
	f5_2_5.AddEmptyMessage(2)
	f5_2.AddMessage(5, f5_2_5)
	f5_2.AddVarint(6, 1)
	f5.AddMessage(2, f5_2)

	// Video info (field 3)
	f5_3 := NewProtoBuilder()
	f5_3_2 := NewProtoBuilder()
	f5_3_2.AddEmptyMessage(3)
	f5_3_2.AddEmptyMessage(4)
	f5_3.AddMessage(2, f5_3_2)

	f5_3_3 := NewProtoBuilder()
	f5_3_3.AddEmptyMessage(2)
	f5_3_3_3 := NewProtoBuilder()
	f5_3_3_3.AddVarint(2, 1)
	f5_3_3.AddMessage(3, f5_3_3_3)
	f5_3.AddMessage(3, f5_3_3)

	f5_3.AddEmptyMessage(4)

	f5_3_5 := NewProtoBuilder()
	f5_3_5_2 := NewProtoBuilder()
	f5_3_5_2.AddVarint(2, 1)
	f5_3_5.AddMessage(2, f5_3_5_2)
	f5_3.AddMessage(5, f5_3_5)

	f5_3.AddEmptyMessage(7)
	f5.AddMessage(3, f5_3)

	// field 4
	f5_4 := NewProtoBuilder()
	f5_4_2 := NewProtoBuilder()
	f5_4_2.AddEmptyMessage(2)
	f5_4.AddMessage(2, f5_4_2)
	f5.AddMessage(4, f5_4)

	// field 5 = micro video
	f5_5 := NewProtoBuilder()
	f5_5_1 := NewProtoBuilder()
	f5_5_1_2 := NewProtoBuilder()
	f5_5_1_2.AddEmptyMessage(3)
	f5_5_1_2.AddEmptyMessage(4)
	f5_5_1.AddMessage(2, f5_5_1_2)
	f5_5_1_3 := NewProtoBuilder()
	f5_5_1_3.AddEmptyMessage(2)
	f5_5_1_3_3 := NewProtoBuilder()
	f5_5_1_3_3.AddVarint(2, 1)
	f5_5_1_3.AddMessage(3, f5_5_1_3_3)
	f5_5_1.AddMessage(3, f5_5_1_3)
	f5_5.AddMessage(1, f5_5_1)
	f5_5.AddVarint(3, 1)
	f5.AddMessage(5, f5_5)

	return f5
}

// buildLocationFieldMask builds location-related field mask
func buildLocationFieldMask() *ProtoBuilder {
	f9 := NewProtoBuilder()
	f9.AddEmptyMessage(2)

	f9_3 := NewProtoBuilder()
	f9_3.AddEmptyMessage(1)
	f9_3.AddEmptyMessage(2)
	f9.AddMessage(3, f9_3)

	f9_4 := NewProtoBuilder()
	f9_4_1 := NewProtoBuilder()
	f9_4_1_3 := NewProtoBuilder()
	f9_4_1_3_1 := NewProtoBuilder()
	f9_4_1_3_1_1 := NewProtoBuilder()
	f9_4_1_3_1_1_5 := NewProtoBuilder()
	f9_4_1_3_1_1_5.AddEmptyMessage(1)
	f9_4_1_3_1_1.AddMessage(5, f9_4_1_3_1_1_5)
	f9_4_1_3_1_1.AddEmptyMessage(6)
	f9_4_1_3_1.AddMessage(1, f9_4_1_3_1_1)
	f9_4_1_3_1.AddEmptyMessage(2)
	f9_4_1_3_1_3 := NewProtoBuilder()
	f9_4_1_3_1_3_1 := NewProtoBuilder()
	f9_4_1_3_1_3_1_5 := NewProtoBuilder()
	f9_4_1_3_1_3_1_5.AddEmptyMessage(1)
	f9_4_1_3_1_3_1.AddMessage(5, f9_4_1_3_1_3_1_5)
	f9_4_1_3_1_3_1.AddEmptyMessage(6)
	f9_4_1_3_1_3.AddMessage(1, f9_4_1_3_1_3_1)
	f9_4_1_3_1_3.AddEmptyMessage(2)
	f9_4_1_3_1.AddMessage(3, f9_4_1_3_1_3)
	f9_4_1_3.AddMessage(1, f9_4_1_3_1)
	f9_4_1.AddMessage(3, f9_4_1_3)

	f9_4_1_4 := NewProtoBuilder()
	f9_4_1_4_1 := NewProtoBuilder()
	f9_4_1_4_1.AddEmptyMessage(2)
	f9_4_1_4.AddMessage(1, f9_4_1_4_1)
	f9_4_1.AddMessage(4, f9_4_1_4)

	f9_4.AddMessage(1, f9_4_1)
	f9.AddMessage(4, f9_4)

	return f9
}

// buildTimestampFieldMask builds timestamp field mask
func buildTimestampFieldMask() *ProtoBuilder {
	f11 := NewProtoBuilder()
	f11.AddEmptyMessage(2)
	f11.AddEmptyMessage(3)
	f11_4 := NewProtoBuilder()
	f11_4_2 := NewProtoBuilder()
	f11_4_2.AddVarint(1, 1)
	f11_4_2.AddVarint(2, 2)
	f11_4.AddMessage(2, f11_4_2)
	f11.AddMessage(4, f11_4)
	return f11
}

// buildItemFieldMask builds the complete item field mask (field 1 of the sync request)
func buildItemFieldMask() *ProtoBuilder {
	mask := NewProtoBuilder()
	mask.AddMessage(1, buildMediaFieldMask())
	mask.AddMessage(5, buildMediaTypeFieldMask())
	mask.AddEmptyMessage(8)
	mask.AddMessage(9, buildLocationFieldMask())
	mask.AddMessage(11, buildTimestampFieldMask())
	mask.AddEmptyMessage(12)

	// field 14 = same as 11
	mask.AddMessage(14, buildTimestampFieldMask())

	// field 15 = thumbnail fields
	f15 := NewProtoBuilder()
	f15.AddEmptyMessage(1)
	f15.AddEmptyMessage(4)
	mask.AddMessage(15, f15)

	// field 17 = same as 15
	mask.AddMessage(17, f15)

	// field 19 = same as 11
	mask.AddMessage(19, buildTimestampFieldMask())

	mask.AddEmptyMessage(22)
	mask.AddEmptyMessage(23)

	return mask
}

// buildSyncOptions builds the sync options part of the request
func buildSyncOptions() *ProtoBuilder {
	f9 := NewProtoBuilder()

	f9_1 := NewProtoBuilder()
	f9_1_2 := NewProtoBuilder()
	f9_1_2.AddEmptyMessage(1)
	f9_1_2.AddEmptyMessage(2)
	f9_1.AddMessage(2, f9_1_2)
	f9.AddMessage(1, f9_1)

	f9_2 := NewProtoBuilder()
	f9_2_3 := NewProtoBuilder()
	f9_2_3.AddVarint(2, 1)
	f9_2.AddMessage(3, f9_2_3)
	f9.AddMessage(2, f9_2)

	f9_3 := NewProtoBuilder()
	f9_3.AddEmptyMessage(2)
	f9.AddMessage(3, f9_3)

	f9.AddEmptyMessage(4)

	f9_7 := NewProtoBuilder()
	f9_7.AddEmptyMessage(1)
	f9.AddMessage(7, f9_7)

	f9_8 := NewProtoBuilder()
	f9_8.AddVarint(1, 2)
	f9_8.AddBytes(2, []byte{0x01, 0x02, 0x03, 0x05, 0x06})
	f9.AddMessage(8, f9_8)

	f9.AddEmptyMessage(9)

	return f9
}

// buildGetLibStateRequest builds the GetLibraryState request body
func buildGetLibStateRequest(stateToken string) []byte {
	// Build the main request content (field 1)
	content := NewProtoBuilder()

	// field 1.1 = item field mask
	content.AddMessage(1, buildItemFieldMask())

	// field 1.2 = collection field mask (simplified)
	collMask := NewProtoBuilder()
	collMask_1 := NewProtoBuilder()
	collMask_1.AddEmptyMessage(2)
	collMask_1.AddEmptyMessage(3)
	collMask_1.AddEmptyMessage(4)
	collMask_1.AddEmptyMessage(5)
	f6 := NewProtoBuilder()
	f6.AddEmptyMessage(1)
	f6.AddEmptyMessage(2)
	f6.AddEmptyMessage(3)
	f6.AddEmptyMessage(4)
	f6.AddEmptyMessage(5)
	f6.AddEmptyMessage(7)
	collMask_1.AddMessage(6, f6)
	collMask_1.AddEmptyMessage(7)
	collMask_1.AddEmptyMessage(8)
	collMask_1.AddEmptyMessage(10)
	collMask_1.AddEmptyMessage(12)
	cm13 := NewProtoBuilder()
	cm13.AddEmptyMessage(2)
	cm13.AddEmptyMessage(3)
	collMask_1.AddMessage(13, cm13)
	cm15 := NewProtoBuilder()
	cm15.AddEmptyMessage(1)
	collMask_1.AddMessage(15, cm15)
	collMask_1.AddEmptyMessage(18)
	collMask.AddMessage(1, collMask_1)
	cm4 := NewProtoBuilder()
	cm4.AddEmptyMessage(1)
	collMask.AddMessage(4, cm4)
	collMask.AddEmptyMessage(9)
	cm11 := NewProtoBuilder()
	cm11_1 := NewProtoBuilder()
	cm11_1.AddEmptyMessage(1)
	cm11_1.AddEmptyMessage(4)
	cm11_1.AddEmptyMessage(5)
	cm11_1.AddEmptyMessage(6)
	cm11_1.AddEmptyMessage(9)
	cm11.AddMessage(1, cm11_1)
	collMask.AddMessage(11, cm11)
	collMask.AddEmptyMessage(17)
	cm18 := NewProtoBuilder()
	cm18.AddEmptyMessage(1)
	cm18_2 := NewProtoBuilder()
	cm18_2.AddEmptyMessage(1)
	cm18.AddMessage(2, cm18_2)
	collMask.AddMessage(18, cm18)
	cm20 := NewProtoBuilder()
	cm20_2 := NewProtoBuilder()
	cm20_2.AddEmptyMessage(1)
	cm20_2.AddEmptyMessage(2)
	cm20.AddMessage(2, cm20_2)
	collMask.AddMessage(20, cm20)
	collMask.AddEmptyMessage(23)
	content.AddMessage(2, collMask)

	// field 1.3 = envelope mask (simplified)
	envMask := NewProtoBuilder()
	envMask.AddEmptyMessage(2)
	em3 := NewProtoBuilder()
	em3.AddEmptyMessage(2)
	em3.AddEmptyMessage(3)
	em3.AddEmptyMessage(7)
	em3.AddEmptyMessage(8)
	em3_14 := NewProtoBuilder()
	em3_14.AddEmptyMessage(1)
	em3.AddMessage(14, em3_14)
	em3.AddEmptyMessage(16)
	em3_17 := NewProtoBuilder()
	em3_17.AddEmptyMessage(2)
	em3.AddMessage(17, em3_17)
	em3.AddEmptyMessage(18)
	em3.AddEmptyMessage(19)
	em3.AddEmptyMessage(20)
	em3.AddEmptyMessage(21)
	em3.AddEmptyMessage(22)
	em3.AddEmptyMessage(23)
	em3_27 := NewProtoBuilder()
	em3_27.AddEmptyMessage(1)
	em3_27_2 := NewProtoBuilder()
	em3_27_2.AddEmptyMessage(1)
	em3_27.AddMessage(2, em3_27_2)
	em3.AddMessage(27, em3_27)
	em3.AddEmptyMessage(29)
	em3.AddEmptyMessage(30)
	em3.AddEmptyMessage(31)
	em3.AddEmptyMessage(32)
	em3.AddEmptyMessage(34)
	em3.AddEmptyMessage(37)
	em3.AddEmptyMessage(38)
	em3.AddEmptyMessage(39)
	em3.AddEmptyMessage(41)
	envMask.AddMessage(3, em3)
	em4 := NewProtoBuilder()
	em4.AddEmptyMessage(2)
	em4_3 := NewProtoBuilder()
	em4_3.AddEmptyMessage(1)
	em4.AddMessage(3, em4_3)
	em4.AddEmptyMessage(4)
	em4_5 := NewProtoBuilder()
	em4_5.AddEmptyMessage(1)
	em4.AddMessage(5, em4_5)
	envMask.AddMessage(4, em4)
	envMask.AddEmptyMessage(7)
	envMask.AddEmptyMessage(12)
	envMask.AddEmptyMessage(13)
	envMask.AddEmptyMessage(15)
	envMask.AddEmptyMessage(18)
	envMask.AddEmptyMessage(20)
	envMask.AddEmptyMessage(24)
	envMask.AddEmptyMessage(25)
	content.AddMessage(3, envMask)

	// field 1.6 = state_token
	if stateToken != "" {
		content.AddString(6, stateToken)
	}

	// field 1.7 = 2
	content.AddVarint(7, 2)

	// field 1.9 = sync options
	content.AddMessage(9, buildSyncOptions())

	// field 1.11 = repeated [1, 2, 6]
	content.AddVarint(11, 1)
	content.AddVarint(11, 2)
	content.AddVarint(11, 6)

	// field 1.12 = filter options
	f12 := NewProtoBuilder()
	f12_2 := NewProtoBuilder()
	f12_2.AddEmptyMessage(1)
	f12_2.AddEmptyMessage(2)
	f12.AddMessage(2, f12_2)
	f12_3 := NewProtoBuilder()
	f12_3.AddEmptyMessage(1)
	f12.AddMessage(3, f12_3)
	f12.AddEmptyMessage(4)
	content.AddMessage(12, f12)

	content.AddEmptyMessage(13)

	// field 1.15
	f15 := NewProtoBuilder()
	f15_3 := NewProtoBuilder()
	f15_3.AddVarint(1, 1)
	f15.AddMessage(3, f15_3)
	content.AddMessage(15, f15)

	// Wrap in outer message
	outer := NewProtoBuilder()
	outer.AddMessage(1, content)

	// field 2 = secondary options
	f2 := NewProtoBuilder()
	f2_1 := NewProtoBuilder()
	f2_1_1 := NewProtoBuilder()
	f2_1_1_1 := NewProtoBuilder()
	f2_1_1_1.AddEmptyMessage(1)
	f2_1_1.AddMessage(1, f2_1_1_1)
	f2_1_1.AddEmptyMessage(2)
	f2_1.AddMessage(1, f2_1_1)
	f2.AddMessage(1, f2_1)
	f2.AddEmptyMessage(2)
	outer.AddMessage(2, f2)

	return outer.Bytes()
}

// buildGetLibPageInitRequest builds the request for library page during init
func buildGetLibPageInitRequest(pageToken string) []byte {
	content := NewProtoBuilder()

	content.AddMessage(1, buildItemFieldMask())

	// Same collection/envelope masks as GetLibState (simplified)
	collMask := NewProtoBuilder()
	collMask_1 := NewProtoBuilder()
	collMask_1.AddEmptyMessage(2)
	collMask_1.AddEmptyMessage(3)
	collMask_1.AddEmptyMessage(4)
	collMask_1.AddEmptyMessage(5)
	f6 := NewProtoBuilder()
	f6.AddEmptyMessage(1)
	f6.AddEmptyMessage(2)
	f6.AddEmptyMessage(3)
	f6.AddEmptyMessage(4)
	f6.AddEmptyMessage(5)
	f6.AddEmptyMessage(7)
	collMask_1.AddMessage(6, f6)
	collMask_1.AddEmptyMessage(7)
	collMask_1.AddEmptyMessage(8)
	collMask_1.AddEmptyMessage(10)
	collMask_1.AddEmptyMessage(12)
	cm13 := NewProtoBuilder()
	cm13.AddEmptyMessage(2)
	cm13.AddEmptyMessage(3)
	collMask_1.AddMessage(13, cm13)
	cm15 := NewProtoBuilder()
	cm15.AddEmptyMessage(1)
	collMask_1.AddMessage(15, cm15)
	collMask_1.AddEmptyMessage(18)
	collMask.AddMessage(1, collMask_1)
	cm4 := NewProtoBuilder()
	cm4.AddEmptyMessage(1)
	collMask.AddMessage(4, cm4)
	collMask.AddEmptyMessage(9)
	collMask.AddEmptyMessage(17)
	cm18 := NewProtoBuilder()
	cm18.AddEmptyMessage(1)
	cm18_2 := NewProtoBuilder()
	cm18_2.AddEmptyMessage(1)
	cm18.AddMessage(2, cm18_2)
	collMask.AddMessage(18, cm18)
	cm20 := NewProtoBuilder()
	cm20_2 := NewProtoBuilder()
	cm20_2.AddEmptyMessage(1)
	cm20_2.AddEmptyMessage(2)
	cm20.AddMessage(2, cm20_2)
	collMask.AddMessage(20, cm20)
	collMask.AddEmptyMessage(23)
	content.AddMessage(2, collMask)

	// Envelope mask (simplified)
	envMask := NewProtoBuilder()
	envMask.AddEmptyMessage(2)
	em3 := NewProtoBuilder()
	em3.AddEmptyMessage(2)
	em3.AddEmptyMessage(3)
	em3.AddEmptyMessage(7)
	em3.AddEmptyMessage(8)
	em3_14 := NewProtoBuilder()
	em3_14.AddEmptyMessage(1)
	em3.AddMessage(14, em3_14)
	em3.AddEmptyMessage(16)
	em3_17 := NewProtoBuilder()
	em3_17.AddEmptyMessage(2)
	em3.AddMessage(17, em3_17)
	em3.AddEmptyMessage(18)
	em3.AddEmptyMessage(19)
	em3.AddEmptyMessage(20)
	em3.AddEmptyMessage(21)
	em3.AddEmptyMessage(22)
	em3.AddEmptyMessage(23)
	em3_27 := NewProtoBuilder()
	em3_27.AddEmptyMessage(1)
	em3_27_2 := NewProtoBuilder()
	em3_27_2.AddEmptyMessage(1)
	em3_27.AddMessage(2, em3_27_2)
	em3.AddMessage(27, em3_27)
	em3.AddEmptyMessage(29)
	em3.AddEmptyMessage(30)
	em3.AddEmptyMessage(31)
	em3.AddEmptyMessage(32)
	em3.AddEmptyMessage(34)
	em3.AddEmptyMessage(37)
	em3.AddEmptyMessage(38)
	em3.AddEmptyMessage(39)
	em3.AddEmptyMessage(41)
	envMask.AddMessage(3, em3)
	em4 := NewProtoBuilder()
	em4.AddEmptyMessage(2)
	em4.AddEmptyMessage(3)
	em4.AddEmptyMessage(4)
	envMask.AddMessage(4, em4)
	envMask.AddEmptyMessage(7)
	envMask.AddEmptyMessage(12)
	envMask.AddEmptyMessage(13)
	envMask.AddEmptyMessage(15)
	envMask.AddEmptyMessage(18)
	envMask.AddEmptyMessage(20)
	envMask.AddEmptyMessage(24)
	envMask.AddEmptyMessage(25)
	content.AddMessage(3, envMask)

	// field 4 = page_token
	if pageToken != "" {
		content.AddString(4, pageToken)
	}

	content.AddVarint(7, 2)

	// Sync options
	f9 := buildSyncOptions()
	// Modify for init: field 8.2 = shorter bytes
	content.AddMessage(9, f9)

	content.AddVarint(11, 1)
	content.AddVarint(11, 2)

	f12 := NewProtoBuilder()
	f12_2 := NewProtoBuilder()
	f12_2.AddEmptyMessage(1)
	f12_2.AddEmptyMessage(2)
	f12.AddMessage(2, f12_2)
	f12_3 := NewProtoBuilder()
	f12_3.AddEmptyMessage(1)
	f12.AddMessage(3, f12_3)
	f12.AddEmptyMessage(4)
	content.AddMessage(12, f12)

	content.AddEmptyMessage(13)

	f15 := NewProtoBuilder()
	f15_3 := NewProtoBuilder()
	f15_3.AddVarint(1, 1)
	f15.AddMessage(3, f15_3)
	content.AddMessage(15, f15)

	// Wrap
	outer := NewProtoBuilder()
	outer.AddMessage(1, content)

	f2 := NewProtoBuilder()
	f2_1 := NewProtoBuilder()
	f2_1_1 := NewProtoBuilder()
	f2_1_1_1 := NewProtoBuilder()
	f2_1_1_1.AddEmptyMessage(1)
	f2_1_1.AddMessage(1, f2_1_1_1)
	f2_1_1.AddEmptyMessage(2)
	f2_1.AddMessage(1, f2_1_1)
	f2.AddMessage(1, f2_1)
	f2.AddEmptyMessage(2)
	outer.AddMessage(2, f2)

	return outer.Bytes()
}

// buildGetLibPageRequest builds the request for library page (delta)
func buildGetLibPageRequest(pageToken, stateToken string) []byte {
	content := NewProtoBuilder()

	content.AddMessage(1, buildItemFieldMask())

	// Collection mask (same as init)
	collMask := NewProtoBuilder()
	collMask_1 := NewProtoBuilder()
	collMask_1.AddEmptyMessage(2)
	collMask_1.AddEmptyMessage(3)
	collMask_1.AddEmptyMessage(4)
	collMask_1.AddEmptyMessage(5)
	f6 := NewProtoBuilder()
	f6.AddEmptyMessage(1)
	f6.AddEmptyMessage(2)
	f6.AddEmptyMessage(3)
	f6.AddEmptyMessage(4)
	f6.AddEmptyMessage(5)
	f6.AddEmptyMessage(7)
	collMask_1.AddMessage(6, f6)
	collMask_1.AddEmptyMessage(7)
	collMask_1.AddEmptyMessage(8)
	collMask_1.AddEmptyMessage(10)
	collMask_1.AddEmptyMessage(12)
	cm13 := NewProtoBuilder()
	cm13.AddEmptyMessage(2)
	cm13.AddEmptyMessage(3)
	collMask_1.AddMessage(13, cm13)
	cm15 := NewProtoBuilder()
	cm15.AddEmptyMessage(1)
	collMask_1.AddMessage(15, cm15)
	collMask_1.AddEmptyMessage(18)
	collMask.AddMessage(1, collMask_1)
	cm4 := NewProtoBuilder()
	cm4.AddEmptyMessage(1)
	collMask.AddMessage(4, cm4)
	collMask.AddEmptyMessage(9)
	collMask.AddEmptyMessage(17)
	cm18 := NewProtoBuilder()
	cm18.AddEmptyMessage(1)
	cm18_2 := NewProtoBuilder()
	cm18_2.AddEmptyMessage(1)
	cm18.AddMessage(2, cm18_2)
	collMask.AddMessage(18, cm18)
	cm20 := NewProtoBuilder()
	cm20_2 := NewProtoBuilder()
	cm20_2.AddEmptyMessage(1)
	cm20_2.AddEmptyMessage(2)
	cm20.AddMessage(2, cm20_2)
	collMask.AddMessage(20, cm20)
	collMask.AddEmptyMessage(23)
	content.AddMessage(2, collMask)

	// Envelope mask
	envMask := NewProtoBuilder()
	envMask.AddEmptyMessage(2)
	em3 := NewProtoBuilder()
	em3.AddEmptyMessage(2)
	em3.AddEmptyMessage(3)
	em3.AddEmptyMessage(7)
	em3.AddEmptyMessage(8)
	em3_14 := NewProtoBuilder()
	em3_14.AddEmptyMessage(1)
	em3.AddMessage(14, em3_14)
	em3.AddEmptyMessage(16)
	em3_17 := NewProtoBuilder()
	em3_17.AddEmptyMessage(2)
	em3.AddMessage(17, em3_17)
	em3.AddEmptyMessage(18)
	em3.AddEmptyMessage(19)
	em3.AddEmptyMessage(20)
	em3.AddEmptyMessage(21)
	em3.AddEmptyMessage(22)
	em3.AddEmptyMessage(23)
	em3_27 := NewProtoBuilder()
	em3_27.AddEmptyMessage(1)
	em3_27_2 := NewProtoBuilder()
	em3_27_2.AddEmptyMessage(1)
	em3_27.AddMessage(2, em3_27_2)
	em3.AddMessage(27, em3_27)
	em3.AddEmptyMessage(29)
	em3.AddEmptyMessage(30)
	em3.AddEmptyMessage(31)
	em3.AddEmptyMessage(32)
	em3.AddEmptyMessage(34)
	em3.AddEmptyMessage(37)
	em3.AddEmptyMessage(38)
	em3.AddEmptyMessage(39)
	em3.AddEmptyMessage(41)
	envMask.AddMessage(3, em3)
	em4 := NewProtoBuilder()
	em4.AddEmptyMessage(2)
	em4.AddEmptyMessage(3)
	em4.AddEmptyMessage(4)
	envMask.AddMessage(4, em4)
	envMask.AddEmptyMessage(7)
	envMask.AddEmptyMessage(12)
	envMask.AddEmptyMessage(13)
	envMask.AddEmptyMessage(15)
	envMask.AddEmptyMessage(18)
	envMask.AddEmptyMessage(20)
	envMask.AddEmptyMessage(24)
	envMask.AddEmptyMessage(25)
	content.AddMessage(3, envMask)

	// field 4 = page_token
	if pageToken != "" {
		content.AddString(4, pageToken)
	}

	// field 6 = state_token
	if stateToken != "" {
		content.AddString(6, stateToken)
	}

	content.AddVarint(7, 2)
	content.AddMessage(9, buildSyncOptions())

	content.AddVarint(11, 1)
	content.AddVarint(11, 2)

	f12 := NewProtoBuilder()
	f12_2 := NewProtoBuilder()
	f12_2.AddEmptyMessage(1)
	f12_2.AddEmptyMessage(2)
	f12.AddMessage(2, f12_2)
	f12_3 := NewProtoBuilder()
	f12_3.AddEmptyMessage(1)
	f12.AddMessage(3, f12_3)
	f12.AddEmptyMessage(4)
	content.AddMessage(12, f12)

	content.AddEmptyMessage(13)

	f15 := NewProtoBuilder()
	f15_3 := NewProtoBuilder()
	f15_3.AddVarint(1, 1)
	f15.AddMessage(3, f15_3)
	content.AddMessage(15, f15)

	outer := NewProtoBuilder()
	outer.AddMessage(1, content)

	f2 := NewProtoBuilder()
	f2_1 := NewProtoBuilder()
	f2_1_1 := NewProtoBuilder()
	f2_1_1_1 := NewProtoBuilder()
	f2_1_1_1.AddEmptyMessage(1)
	f2_1_1.AddMessage(1, f2_1_1_1)
	f2_1_1.AddEmptyMessage(2)
	f2_1.AddMessage(1, f2_1_1)
	f2.AddMessage(1, f2_1)
	f2.AddEmptyMessage(2)
	outer.AddMessage(2, f2)

	return outer.Bytes()
}

// buildHashCheckRequest builds the request to check if a file exists by hash
func buildHashCheckRequest(sha1Hash []byte) []byte {
	outer := NewProtoBuilder()

	f1 := NewProtoBuilder()
	f1_1 := NewProtoBuilder()
	f1_1.AddBytes(1, sha1Hash)
	f1.AddMessage(1, f1_1)
	f1.AddEmptyMessage(2)
	outer.AddMessage(1, f1)

	return outer.Bytes()
}

// buildCommitUploadRequest builds the commit upload request
func buildCommitUploadRequest(uploadResponse []byte, fileName string, sha1Hash []byte, model, make_ string) []byte {
	outer := NewProtoBuilder()

	f1 := NewProtoBuilder()

	// field 1.1 = upload response token (raw bytes from upload response)
	f1.AddBytes(1, uploadResponse)

	// field 1.2 = file_name
	f1.AddString(2, fileName)

	// field 1.3 = sha1_hash
	f1.AddBytes(3, sha1Hash)

	// field 1.4 = timestamp info
	f1_4 := NewProtoBuilder()
	f1_4.AddVarint(1, uint64(time.Now().Unix()))
	f1_4.AddVarint(2, 46000000)
	f1.AddMessage(4, f1_4)

	// field 1.7 = quality (3 = original)
	f1.AddVarint(7, 3)

	// field 1.10 = 1
	f1.AddVarint(10, 1)

	// field 1.17 = 0
	f1.AddVarint(17, 0)

	outer.AddMessage(1, f1)

	// field 2 = device info
	f2 := NewProtoBuilder()
	f2.AddString(3, model)
	f2.AddString(4, make_)
	f2.AddVarint(5, androidAPIVersion)
	outer.AddMessage(2, f2)

	// field 3 = bytes [1, 3]
	outer.AddBytes(3, []byte{1, 3})

	return outer.Bytes()
}

// buildMoveToTrashRequest builds the move to trash request
func buildMoveToTrashRequest(dedupKeys []string) []byte {
	b := NewProtoBuilder()
	b.AddVarint(2, 1)
	for _, key := range dedupKeys {
		b.AddString(3, key)
	}
	b.AddVarint(4, 1)

	f8 := NewProtoBuilder()
	f8_4 := NewProtoBuilder()
	f8_4.AddEmptyMessage(2)
	f8_4_3 := NewProtoBuilder()
	f8_4_3.AddEmptyMessage(1)
	f8_4.AddMessage(3, f8_4_3)
	f8_4.AddEmptyMessage(4)
	f8_4_5 := NewProtoBuilder()
	f8_4_5.AddEmptyMessage(1)
	f8_4.AddMessage(5, f8_4_5)
	f8.AddMessage(4, f8_4)
	b.AddMessage(8, f8)

	f9 := NewProtoBuilder()
	f9.AddVarint(1, 5)
	f9_2 := NewProtoBuilder()
	f9_2.AddVarint(1, clientVersionCode)
	f9_2.AddString(2, "28")
	f9.AddMessage(2, f9_2)
	b.AddMessage(9, f9)

	return b.Bytes()
}

// buildGetDownloadURLsRequest builds the get download URLs request
func buildGetDownloadURLsRequest(mediaKey string) []byte {
	outer := NewProtoBuilder()

	// field 1 = media key wrapper
	f1 := NewProtoBuilder()
	f1_1 := NewProtoBuilder()
	f1_1.AddString(1, mediaKey)
	f1.AddMessage(1, f1_1)
	outer.AddMessage(1, f1)

	// field 2 = download options
	f2 := NewProtoBuilder()

	f2_1 := NewProtoBuilder()
	f2_1_7 := NewProtoBuilder()
	f2_1_7.AddEmptyMessage(2)
	f2_1.AddMessage(7, f2_1_7)
	f2.AddMessage(1, f2_1)

	f2_5 := NewProtoBuilder()
	f2_5.AddEmptyMessage(2)
	f2_5.AddEmptyMessage(3)
	f2_5_5 := NewProtoBuilder()
	f2_5_5.AddEmptyMessage(1)
	f2_5_5.AddVarint(3, 0)
	f2_5.AddMessage(5, f2_5_5)
	f2.AddMessage(5, f2_5)

	outer.AddMessage(2, f2)

	return outer.Bytes()
}
