package erref

type NtStatus uint32

func (e NtStatus) Error() string {
	return ntStatusStrings[e]
}

const (
	STATUS_SUCCESS                                                     NtStatus = 0x00000000
	STATUS_WAIT_0                                                      NtStatus = 0x00000000
	STATUS_WAIT_1                                                      NtStatus = 0x00000001
	STATUS_WAIT_2                                                      NtStatus = 0x00000002
	STATUS_WAIT_3                                                      NtStatus = 0x00000003
	STATUS_WAIT_63                                                     NtStatus = 0x0000003F
	STATUS_ABANDONED                                                   NtStatus = 0x00000080
	STATUS_ABANDONED_WAIT_0                                            NtStatus = 0x00000080
	STATUS_ABANDONED_WAIT_63                                           NtStatus = 0x000000BF
	STATUS_USER_APC                                                    NtStatus = 0x000000C0
	STATUS_ALERTED                                                     NtStatus = 0x00000101
	STATUS_TIMEOUT                                                     NtStatus = 0x00000102
	STATUS_PENDING                                                     NtStatus = 0x00000103
	STATUS_REPARSE                                                     NtStatus = 0x00000104
	STATUS_MORE_ENTRIES                                                NtStatus = 0x00000105
	STATUS_NOT_ALL_ASSIGNED                                            NtStatus = 0x00000106
	STATUS_SOME_NOT_MAPPED                                             NtStatus = 0x00000107
	STATUS_OPLOCK_BREAK_IN_PROGRESS                                    NtStatus = 0x00000108
	STATUS_VOLUME_MOUNTED                                              NtStatus = 0x00000109
	STATUS_RXACT_COMMITTED                                             NtStatus = 0x0000010A
	STATUS_NOTIFY_CLEANUP                                              NtStatus = 0x0000010B
	STATUS_NOTIFY_ENUM_DIR                                             NtStatus = 0x0000010C
	STATUS_NO_QUOTAS_FOR_ACCOUNT                                       NtStatus = 0x0000010D
	STATUS_PRIMARY_TRANSPORT_CONNECT_FAILED                            NtStatus = 0x0000010E
	STATUS_PAGE_FAULT_TRANSITION                                       NtStatus = 0x00000110
	STATUS_PAGE_FAULT_DEMAND_ZERO                                      NtStatus = 0x00000111
	STATUS_PAGE_FAULT_COPY_ON_WRITE                                    NtStatus = 0x00000112
	STATUS_PAGE_FAULT_GUARD_PAGE                                       NtStatus = 0x00000113
	STATUS_PAGE_FAULT_PAGING_FILE                                      NtStatus = 0x00000114
	STATUS_CACHE_PAGE_LOCKED                                           NtStatus = 0x00000115
	STATUS_CRASH_DUMP                                                  NtStatus = 0x00000116
	STATUS_BUFFER_ALL_ZEROS                                            NtStatus = 0x00000117
	STATUS_REPARSE_OBJECT                                              NtStatus = 0x00000118
	STATUS_RESOURCE_REQUIREMENTS_CHANGED                               NtStatus = 0x00000119
	STATUS_TRANSLATION_COMPLETE                                        NtStatus = 0x00000120
	STATUS_DS_MEMBERSHIP_EVALUATED_LOCALLY                             NtStatus = 0x00000121
	STATUS_NOTHING_TO_TERMINATE                                        NtStatus = 0x00000122
	STATUS_PROCESS_NOT_IN_JOB                                          NtStatus = 0x00000123
	STATUS_PROCESS_IN_JOB                                              NtStatus = 0x00000124
	STATUS_VOLSNAP_HIBERNATE_READY                                     NtStatus = 0x00000125
	STATUS_FSFILTER_OP_COMPLETED_SUCCESSFULLY                          NtStatus = 0x00000126
	STATUS_INTERRUPT_VECTOR_ALREADY_CONNECTED                          NtStatus = 0x00000127
	STATUS_INTERRUPT_STILL_CONNECTED                                   NtStatus = 0x00000128
	STATUS_PROCESS_CLONED                                              NtStatus = 0x00000129
	STATUS_FILE_LOCKED_WITH_ONLY_READERS                               NtStatus = 0x0000012A
	STATUS_FILE_LOCKED_WITH_WRITERS                                    NtStatus = 0x0000012B
	STATUS_RESOURCEMANAGER_READ_ONLY                                   NtStatus = 0x00000202
	STATUS_WAIT_FOR_OPLOCK                                             NtStatus = 0x00000367
	DBG_EXCEPTION_HANDLED                                              NtStatus = 0x00010001
	DBG_CONTINUE                                                       NtStatus = 0x00010002
	STATUS_FLT_IO_COMPLETE                                             NtStatus = 0x001C0001
	STATUS_FILE_NOT_AVAILABLE                                          NtStatus = 0xC0000467
	STATUS_CALLBACK_RETURNED_THREAD_AFFINITY                           NtStatus = 0xC0000721
	STATUS_OBJECT_NAME_EXISTS                                          NtStatus = 0x40000000
	STATUS_THREAD_WAS_SUSPENDED                                        NtStatus = 0x40000001
	STATUS_WORKING_SET_LIMIT_RANGE                                     NtStatus = 0x40000002
	STATUS_IMAGE_NOT_AT_BASE                                           NtStatus = 0x40000003
	STATUS_RXACT_STATE_CREATED                                         NtStatus = 0x40000004
	STATUS_SEGMENT_NOTIFICATION                                        NtStatus = 0x40000005
	STATUS_LOCAL_USER_SESSION_KEY                                      NtStatus = 0x40000006
	STATUS_BAD_CURRENT_DIRECTORY                                       NtStatus = 0x40000007
	STATUS_SERIAL_MORE_WRITES                                          NtStatus = 0x40000008
	STATUS_REGISTRY_RECOVERED                                          NtStatus = 0x40000009
	STATUS_FT_READ_RECOVERY_FROM_BACKUP                                NtStatus = 0x4000000A
	STATUS_FT_WRITE_RECOVERY                                           NtStatus = 0x4000000B
	STATUS_SERIAL_COUNTER_TIMEOUT                                      NtStatus = 0x4000000C
	STATUS_NULL_LM_PASSWORD                                            NtStatus = 0x4000000D
	STATUS_IMAGE_MACHINE_TYPE_MISMATCH                                 NtStatus = 0x4000000E
	STATUS_RECEIVE_PARTIAL                                             NtStatus = 0x4000000F
	STATUS_RECEIVE_EXPEDITED                                           NtStatus = 0x40000010
	STATUS_RECEIVE_PARTIAL_EXPEDITED                                   NtStatus = 0x40000011
	STATUS_EVENT_DONE                                                  NtStatus = 0x40000012
	STATUS_EVENT_PENDING                                               NtStatus = 0x40000013
	STATUS_CHECKING_FILE_SYSTEM                                        NtStatus = 0x40000014
	STATUS_FATAL_APP_EXIT                                              NtStatus = 0x40000015
	STATUS_PREDEFINED_HANDLE                                           NtStatus = 0x40000016
	STATUS_WAS_UNLOCKED                                                NtStatus = 0x40000017
	STATUS_SERVICE_NOTIFICATION                                        NtStatus = 0x40000018
	STATUS_WAS_LOCKED                                                  NtStatus = 0x40000019
	STATUS_LOG_HARD_ERROR                                              NtStatus = 0x4000001A
	STATUS_ALREADY_WIN32                                               NtStatus = 0x4000001B
	STATUS_WX86_UNSIMULATE                                             NtStatus = 0x4000001C
	STATUS_WX86_CONTINUE                                               NtStatus = 0x4000001D
	STATUS_WX86_SINGLE_STEP                                            NtStatus = 0x4000001E
	STATUS_WX86_BREAKPOINT                                             NtStatus = 0x4000001F
	STATUS_WX86_EXCEPTION_CONTINUE                                     NtStatus = 0x40000020
	STATUS_WX86_EXCEPTION_LASTCHANCE                                   NtStatus = 0x40000021
	STATUS_WX86_EXCEPTION_CHAIN                                        NtStatus = 0x40000022
	STATUS_IMAGE_MACHINE_TYPE_MISMATCH_EXE                             NtStatus = 0x40000023
	STATUS_NO_YIELD_PERFORMED                                          NtStatus = 0x40000024
	STATUS_TIMER_RESUME_IGNORED                                        NtStatus = 0x40000025
	STATUS_ARBITRATION_UNHANDLED                                       NtStatus = 0x40000026
	STATUS_CARDBUS_NOT_SUPPORTED                                       NtStatus = 0x40000027
	STATUS_WX86_CREATEWX86TIB                                          NtStatus = 0x40000028
	STATUS_MP_PROCESSOR_MISMATCH                                       NtStatus = 0x40000029
	STATUS_HIBERNATED                                                  NtStatus = 0x4000002A
	STATUS_RESUME_HIBERNATION                                          NtStatus = 0x4000002B
	STATUS_FIRMWARE_UPDATED                                            NtStatus = 0x4000002C
	STATUS_DRIVERS_LEAKING_LOCKED_PAGES                                NtStatus = 0x4000002D
	STATUS_MESSAGE_RETRIEVED                                           NtStatus = 0x4000002E
	STATUS_SYSTEM_POWERSTATE_TRANSITION                                NtStatus = 0x4000002F
	STATUS_ALPC_CHECK_COMPLETION_LIST                                  NtStatus = 0x40000030
	STATUS_SYSTEM_POWERSTATE_COMPLEX_TRANSITION                        NtStatus = 0x40000031
	STATUS_ACCESS_AUDIT_BY_POLICY                                      NtStatus = 0x40000032
	STATUS_ABANDON_HIBERFILE                                           NtStatus = 0x40000033
	STATUS_BIZRULES_NOT_ENABLED                                        NtStatus = 0x40000034
	STATUS_WAKE_SYSTEM                                                 NtStatus = 0x40000294
	STATUS_DS_SHUTTING_DOWN                                            NtStatus = 0x40000370
	DBG_REPLY_LATER                                                    NtStatus = 0x40010001
	DBG_UNABLE_TO_PROVIDE_HANDLE                                       NtStatus = 0x40010002
	DBG_TERMINATE_THREAD                                               NtStatus = 0x40010003
	DBG_TERMINATE_PROCESS                                              NtStatus = 0x40010004
	DBG_CONTROL_C                                                      NtStatus = 0x40010005
	DBG_PRINTEXCEPTION_C                                               NtStatus = 0x40010006
	DBG_RIPEXCEPTION                                                   NtStatus = 0x40010007
	DBG_CONTROL_BREAK                                                  NtStatus = 0x40010008
	DBG_COMMAND_EXCEPTION                                              NtStatus = 0x40010009
	RPC_NT_UUID_LOCAL_ONLY                                             NtStatus = 0x40020056
	RPC_NT_SEND_INCOMPLETE                                             NtStatus = 0x400200AF
	STATUS_CTX_CDM_CONNECT                                             NtStatus = 0x400A0004
	STATUS_CTX_CDM_DISCONNECT                                          NtStatus = 0x400A0005
	STATUS_SXS_RELEASE_ACTIVATION_CONTEXT                              NtStatus = 0x4015000D
	STATUS_RECOVERY_NOT_NEEDED                                         NtStatus = 0x40190034
	STATUS_RM_ALREADY_STARTED                                          NtStatus = 0x40190035
	STATUS_LOG_NO_RESTART                                              NtStatus = 0x401A000C
	STATUS_VIDEO_DRIVER_DEBUG_REPORT_REQUEST                           NtStatus = 0x401B00EC
	STATUS_GRAPHICS_PARTIAL_DATA_POPULATED                             NtStatus = 0x401E000A
	STATUS_GRAPHICS_DRIVER_MISMATCH                                    NtStatus = 0x401E0117
	STATUS_GRAPHICS_MODE_NOT_PINNED                                    NtStatus = 0x401E0307
	STATUS_GRAPHICS_NO_PREFERRED_MODE                                  NtStatus = 0x401E031E
	STATUS_GRAPHICS_DATASET_IS_EMPTY                                   NtStatus = 0x401E034B
	STATUS_GRAPHICS_NO_MORE_ELEMENTS_IN_DATASET                        NtStatus = 0x401E034C
	STATUS_GRAPHICS_PATH_CONTENT_GEOMETRY_TRANSFORMATION_NOT_PINNED    NtStatus = 0x401E0351
	STATUS_GRAPHICS_UNKNOWN_CHILD_STATUS                               NtStatus = 0x401E042F
	STATUS_GRAPHICS_LEADLINK_START_DEFERRED                            NtStatus = 0x401E0437
	STATUS_GRAPHICS_POLLING_TOO_FREQUENTLY                             NtStatus = 0x401E0439
	STATUS_GRAPHICS_START_DEFERRED                                     NtStatus = 0x401E043A
	STATUS_NDIS_INDICATION_REQUIRED                                    NtStatus = 0x40230001
	STATUS_GUARD_PAGE_VIOLATION                                        NtStatus = 0x80000001
	STATUS_DATATYPE_MISALIGNMENT                                       NtStatus = 0x80000002
	STATUS_BREAKPOINT                                                  NtStatus = 0x80000003
	STATUS_SINGLE_STEP                                                 NtStatus = 0x80000004
	STATUS_BUFFER_OVERFLOW                                             NtStatus = 0x80000005
	STATUS_NO_MORE_FILES                                               NtStatus = 0x80000006
	STATUS_WAKE_SYSTEM_DEBUGGER                                        NtStatus = 0x80000007
	STATUS_HANDLES_CLOSED                                              NtStatus = 0x8000000A
	STATUS_NO_INHERITANCE                                              NtStatus = 0x8000000B
	STATUS_GUID_SUBSTITUTION_MADE                                      NtStatus = 0x8000000C
	STATUS_PARTIAL_COPY                                                NtStatus = 0x8000000D
	STATUS_DEVICE_PAPER_EMPTY                                          NtStatus = 0x8000000E
	STATUS_DEVICE_POWERED_OFF                                          NtStatus = 0x8000000F
	STATUS_DEVICE_OFF_LINE                                             NtStatus = 0x80000010
	STATUS_DEVICE_BUSY                                                 NtStatus = 0x80000011
	STATUS_NO_MORE_EAS                                                 NtStatus = 0x80000012
	STATUS_INVALID_EA_NAME                                             NtStatus = 0x80000013
	STATUS_EA_LIST_INCONSISTENT                                        NtStatus = 0x80000014
	STATUS_INVALID_EA_FLAG                                             NtStatus = 0x80000015
	STATUS_VERIFY_REQUIRED                                             NtStatus = 0x80000016
	STATUS_EXTRANEOUS_INFORMATION                                      NtStatus = 0x80000017
	STATUS_RXACT_COMMIT_NECESSARY                                      NtStatus = 0x80000018
	STATUS_NO_MORE_ENTRIES                                             NtStatus = 0x8000001A
	STATUS_FILEMARK_DETECTED                                           NtStatus = 0x8000001B
	STATUS_MEDIA_CHANGED                                               NtStatus = 0x8000001C
	STATUS_BUS_RESET                                                   NtStatus = 0x8000001D
	STATUS_END_OF_MEDIA                                                NtStatus = 0x8000001E
	STATUS_BEGINNING_OF_MEDIA                                          NtStatus = 0x8000001F
	STATUS_MEDIA_CHECK                                                 NtStatus = 0x80000020
	STATUS_SETMARK_DETECTED                                            NtStatus = 0x80000021
	STATUS_NO_DATA_DETECTED                                            NtStatus = 0x80000022
	STATUS_REDIRECTOR_HAS_OPEN_HANDLES                                 NtStatus = 0x80000023
	STATUS_SERVER_HAS_OPEN_HANDLES                                     NtStatus = 0x80000024
	STATUS_ALREADY_DISCONNECTED                                        NtStatus = 0x80000025
	STATUS_LONGJUMP                                                    NtStatus = 0x80000026
	STATUS_CLEANER_CARTRIDGE_INSTALLED                                 NtStatus = 0x80000027
	STATUS_PLUGPLAY_QUERY_VETOED                                       NtStatus = 0x80000028
	STATUS_UNWIND_CONSOLIDATE                                          NtStatus = 0x80000029
	STATUS_REGISTRY_HIVE_RECOVERED                                     NtStatus = 0x8000002A
	STATUS_DLL_MIGHT_BE_INSECURE                                       NtStatus = 0x8000002B
	STATUS_DLL_MIGHT_BE_INCOMPATIBLE                                   NtStatus = 0x8000002C
	STATUS_STOPPED_ON_SYMLINK                                          NtStatus = 0x8000002D
	STATUS_DEVICE_REQUIRES_CLEANING                                    NtStatus = 0x80000288
	STATUS_DEVICE_DOOR_OPEN                                            NtStatus = 0x80000289
	STATUS_DATA_LOST_REPAIR                                            NtStatus = 0x80000803
	DBG_EXCEPTION_NOT_HANDLED                                          NtStatus = 0x80010001
	STATUS_CLUSTER_NODE_ALREADY_UP                                     NtStatus = 0x80130001
	STATUS_CLUSTER_NODE_ALREADY_DOWN                                   NtStatus = 0x80130002
	STATUS_CLUSTER_NETWORK_ALREADY_ONLINE                              NtStatus = 0x80130003
	STATUS_CLUSTER_NETWORK_ALREADY_OFFLINE                             NtStatus = 0x80130004
	STATUS_CLUSTER_NODE_ALREADY_MEMBER                                 NtStatus = 0x80130005
	STATUS_COULD_NOT_RESIZE_LOG                                        NtStatus = 0x80190009
	STATUS_NO_TXF_METADATA                                             NtStatus = 0x80190029
	STATUS_CANT_RECOVER_WITH_HANDLE_OPEN                               NtStatus = 0x80190031
	STATUS_TXF_METADATA_ALREADY_PRESENT                                NtStatus = 0x80190041
	STATUS_TRANSACTION_SCOPE_CALLBACKS_NOT_SET                         NtStatus = 0x80190042
	STATUS_VIDEO_HUNG_DISPLAY_DRIVER_THREAD_RECOVERED                  NtStatus = 0x801B00EB
	STATUS_FLT_BUFFER_TOO_SMALL                                        NtStatus = 0x801C0001
	STATUS_FVE_PARTIAL_METADATA                                        NtStatus = 0x80210001
	STATUS_FVE_TRANSIENT_STATE                                         NtStatus = 0x80210002
	STATUS_UNSUCCESSFUL                                                NtStatus = 0xC0000001
	STATUS_NOT_IMPLEMENTED                                             NtStatus = 0xC0000002
	STATUS_INVALID_INFO_CLASS                                          NtStatus = 0xC0000003
	STATUS_INFO_LENGTH_MISMATCH                                        NtStatus = 0xC0000004
	STATUS_ACCESS_VIOLATION                                            NtStatus = 0xC0000005
	STATUS_IN_PAGE_ERROR                                               NtStatus = 0xC0000006
	STATUS_PAGEFILE_QUOTA                                              NtStatus = 0xC0000007
	STATUS_INVALID_HANDLE                                              NtStatus = 0xC0000008
	STATUS_BAD_INITIAL_STACK                                           NtStatus = 0xC0000009
	STATUS_BAD_INITIAL_PC                                              NtStatus = 0xC000000A
	STATUS_INVALID_CID                                                 NtStatus = 0xC000000B
	STATUS_TIMER_NOT_CANCELED                                          NtStatus = 0xC000000C
	STATUS_INVALID_PARAMETER                                           NtStatus = 0xC000000D
	STATUS_NO_SUCH_DEVICE                                              NtStatus = 0xC000000E
	STATUS_NO_SUCH_FILE                                                NtStatus = 0xC000000F
	STATUS_INVALID_DEVICE_REQUEST                                      NtStatus = 0xC0000010
	STATUS_END_OF_FILE                                                 NtStatus = 0xC0000011
	STATUS_WRONG_VOLUME                                                NtStatus = 0xC0000012
	STATUS_NO_MEDIA_IN_DEVICE                                          NtStatus = 0xC0000013
	STATUS_UNRECOGNIZED_MEDIA                                          NtStatus = 0xC0000014
	STATUS_NONEXISTENT_SECTOR                                          NtStatus = 0xC0000015
	STATUS_MORE_PROCESSING_REQUIRED                                    NtStatus = 0xC0000016
	STATUS_NO_MEMORY                                                   NtStatus = 0xC0000017
	STATUS_CONFLICTING_ADDRESSES                                       NtStatus = 0xC0000018
	STATUS_NOT_MAPPED_VIEW                                             NtStatus = 0xC0000019
	STATUS_UNABLE_TO_FREE_VM                                           NtStatus = 0xC000001A
	STATUS_UNABLE_TO_DELETE_SECTION                                    NtStatus = 0xC000001B
	STATUS_INVALID_SYSTEM_SERVICE                                      NtStatus = 0xC000001C
	STATUS_ILLEGAL_INSTRUCTION                                         NtStatus = 0xC000001D
	STATUS_INVALID_LOCK_SEQUENCE                                       NtStatus = 0xC000001E
	STATUS_INVALID_VIEW_SIZE                                           NtStatus = 0xC000001F
	STATUS_INVALID_FILE_FOR_SECTION                                    NtStatus = 0xC0000020
	STATUS_ALREADY_COMMITTED                                           NtStatus = 0xC0000021
	STATUS_ACCESS_DENIED                                               NtStatus = 0xC0000022
	STATUS_BUFFER_TOO_SMALL                                            NtStatus = 0xC0000023
	STATUS_OBJECT_TYPE_MISMATCH                                        NtStatus = 0xC0000024
	STATUS_NONCONTINUABLE_EXCEPTION                                    NtStatus = 0xC0000025
	STATUS_INVALID_DISPOSITION                                         NtStatus = 0xC0000026
	STATUS_UNWIND                                                      NtStatus = 0xC0000027
	STATUS_BAD_STACK                                                   NtStatus = 0xC0000028
	STATUS_INVALID_UNWIND_TARGET                                       NtStatus = 0xC0000029
	STATUS_NOT_LOCKED                                                  NtStatus = 0xC000002A
	STATUS_PARITY_ERROR                                                NtStatus = 0xC000002B
	STATUS_UNABLE_TO_DECOMMIT_VM                                       NtStatus = 0xC000002C
	STATUS_NOT_COMMITTED                                               NtStatus = 0xC000002D
	STATUS_INVALID_PORT_ATTRIBUTES                                     NtStatus = 0xC000002E
	STATUS_PORT_MESSAGE_TOO_LONG                                       NtStatus = 0xC000002F
	STATUS_INVALID_PARAMETER_MIX                                       NtStatus = 0xC0000030
	STATUS_INVALID_QUOTA_LOWER                                         NtStatus = 0xC0000031
	STATUS_DISK_CORRUPT_ERROR                                          NtStatus = 0xC0000032
	STATUS_OBJECT_NAME_INVALID                                         NtStatus = 0xC0000033
	STATUS_OBJECT_NAME_NOT_FOUND                                       NtStatus = 0xC0000034
	STATUS_OBJECT_NAME_COLLISION                                       NtStatus = 0xC0000035
	STATUS_PORT_DISCONNECTED                                           NtStatus = 0xC0000037
	STATUS_DEVICE_ALREADY_ATTACHED                                     NtStatus = 0xC0000038
	STATUS_OBJECT_PATH_INVALID                                         NtStatus = 0xC0000039
	STATUS_OBJECT_PATH_NOT_FOUND                                       NtStatus = 0xC000003A
	STATUS_OBJECT_PATH_SYNTAX_BAD                                      NtStatus = 0xC000003B
	STATUS_DATA_OVERRUN                                                NtStatus = 0xC000003C
	STATUS_DATA_LATE_ERROR                                             NtStatus = 0xC000003D
	STATUS_DATA_ERROR                                                  NtStatus = 0xC000003E
	STATUS_CRC_ERROR                                                   NtStatus = 0xC000003F
	STATUS_SECTION_TOO_BIG                                             NtStatus = 0xC0000040
	STATUS_PORT_CONNECTION_REFUSED                                     NtStatus = 0xC0000041
	STATUS_INVALID_PORT_HANDLE                                         NtStatus = 0xC0000042
	STATUS_SHARING_VIOLATION                                           NtStatus = 0xC0000043
	STATUS_QUOTA_EXCEEDED                                              NtStatus = 0xC0000044
	STATUS_INVALID_PAGE_PROTECTION                                     NtStatus = 0xC0000045
	STATUS_MUTANT_NOT_OWNED                                            NtStatus = 0xC0000046
	STATUS_SEMAPHORE_LIMIT_EXCEEDED                                    NtStatus = 0xC0000047
	STATUS_PORT_ALREADY_SET                                            NtStatus = 0xC0000048
	STATUS_SECTION_NOT_IMAGE                                           NtStatus = 0xC0000049
	STATUS_SUSPEND_COUNT_EXCEEDED                                      NtStatus = 0xC000004A
	STATUS_THREAD_IS_TERMINATING                                       NtStatus = 0xC000004B
	STATUS_BAD_WORKING_SET_LIMIT                                       NtStatus = 0xC000004C
	STATUS_INCOMPATIBLE_FILE_MAP                                       NtStatus = 0xC000004D
	STATUS_SECTION_PROTECTION                                          NtStatus = 0xC000004E
	STATUS_EAS_NOT_SUPPORTED                                           NtStatus = 0xC000004F
	STATUS_EA_TOO_LARGE                                                NtStatus = 0xC0000050
	STATUS_NONEXISTENT_EA_ENTRY                                        NtStatus = 0xC0000051
	STATUS_NO_EAS_ON_FILE                                              NtStatus = 0xC0000052
	STATUS_EA_CORRUPT_ERROR                                            NtStatus = 0xC0000053
	STATUS_FILE_LOCK_CONFLICT                                          NtStatus = 0xC0000054
	STATUS_LOCK_NOT_GRANTED                                            NtStatus = 0xC0000055
	STATUS_DELETE_PENDING                                              NtStatus = 0xC0000056
	STATUS_CTL_FILE_NOT_SUPPORTED                                      NtStatus = 0xC0000057
	STATUS_UNKNOWN_REVISION                                            NtStatus = 0xC0000058
	STATUS_REVISION_MISMATCH                                           NtStatus = 0xC0000059
	STATUS_INVALID_OWNER                                               NtStatus = 0xC000005A
	STATUS_INVALID_PRIMARY_GROUP                                       NtStatus = 0xC000005B
	STATUS_NO_IMPERSONATION_TOKEN                                      NtStatus = 0xC000005C
	STATUS_CANT_DISABLE_MANDATORY                                      NtStatus = 0xC000005D
	STATUS_NO_LOGON_SERVERS                                            NtStatus = 0xC000005E
	STATUS_NO_SUCH_LOGON_SESSION                                       NtStatus = 0xC000005F
	STATUS_NO_SUCH_PRIVILEGE                                           NtStatus = 0xC0000060
	STATUS_PRIVILEGE_NOT_HELD                                          NtStatus = 0xC0000061
	STATUS_INVALID_ACCOUNT_NAME                                        NtStatus = 0xC0000062
	STATUS_USER_EXISTS                                                 NtStatus = 0xC0000063
	STATUS_NO_SUCH_USER                                                NtStatus = 0xC0000064
	STATUS_GROUP_EXISTS                                                NtStatus = 0xC0000065
	STATUS_NO_SUCH_GROUP                                               NtStatus = 0xC0000066
	STATUS_MEMBER_IN_GROUP                                             NtStatus = 0xC0000067
	STATUS_MEMBER_NOT_IN_GROUP                                         NtStatus = 0xC0000068
	STATUS_LAST_ADMIN                                                  NtStatus = 0xC0000069
	STATUS_WRONG_PASSWORD                                              NtStatus = 0xC000006A
	STATUS_ILL_FORMED_PASSWORD                                         NtStatus = 0xC000006B
	STATUS_PASSWORD_RESTRICTION                                        NtStatus = 0xC000006C
	STATUS_LOGON_FAILURE                                               NtStatus = 0xC000006D
	STATUS_ACCOUNT_RESTRICTION                                         NtStatus = 0xC000006E
	STATUS_INVALID_LOGON_HOURS                                         NtStatus = 0xC000006F
	STATUS_INVALID_WORKSTATION                                         NtStatus = 0xC0000070
	STATUS_PASSWORD_EXPIRED                                            NtStatus = 0xC0000071
	STATUS_ACCOUNT_DISABLED                                            NtStatus = 0xC0000072
	STATUS_NONE_MAPPED                                                 NtStatus = 0xC0000073
	STATUS_TOO_MANY_LUIDS_REQUESTED                                    NtStatus = 0xC0000074
	STATUS_LUIDS_EXHAUSTED                                             NtStatus = 0xC0000075
	STATUS_INVALID_SUB_AUTHORITY                                       NtStatus = 0xC0000076
	STATUS_INVALID_ACL                                                 NtStatus = 0xC0000077
	STATUS_INVALID_SID                                                 NtStatus = 0xC0000078
	STATUS_INVALID_SECURITY_DESCR                                      NtStatus = 0xC0000079
	STATUS_PROCEDURE_NOT_FOUND                                         NtStatus = 0xC000007A
	STATUS_INVALID_IMAGE_FORMAT                                        NtStatus = 0xC000007B
	STATUS_NO_TOKEN                                                    NtStatus = 0xC000007C
	STATUS_BAD_INHERITANCE_ACL                                         NtStatus = 0xC000007D
	STATUS_RANGE_NOT_LOCKED                                            NtStatus = 0xC000007E
	STATUS_DISK_FULL                                                   NtStatus = 0xC000007F
	STATUS_SERVER_DISABLED                                             NtStatus = 0xC0000080
	STATUS_SERVER_NOT_DISABLED                                         NtStatus = 0xC0000081
	STATUS_TOO_MANY_GUIDS_REQUESTED                                    NtStatus = 0xC0000082
	STATUS_GUIDS_EXHAUSTED                                             NtStatus = 0xC0000083
	STATUS_INVALID_ID_AUTHORITY                                        NtStatus = 0xC0000084
	STATUS_AGENTS_EXHAUSTED                                            NtStatus = 0xC0000085
	STATUS_INVALID_VOLUME_LABEL                                        NtStatus = 0xC0000086
	STATUS_SECTION_NOT_EXTENDED                                        NtStatus = 0xC0000087
	STATUS_NOT_MAPPED_DATA                                             NtStatus = 0xC0000088
	STATUS_RESOURCE_DATA_NOT_FOUND                                     NtStatus = 0xC0000089
	STATUS_RESOURCE_TYPE_NOT_FOUND                                     NtStatus = 0xC000008A
	STATUS_RESOURCE_NAME_NOT_FOUND                                     NtStatus = 0xC000008B
	STATUS_ARRAY_BOUNDS_EXCEEDED                                       NtStatus = 0xC000008C
	STATUS_FLOAT_DENORMAL_OPERAND                                      NtStatus = 0xC000008D
	STATUS_FLOAT_DIVIDE_BY_ZERO                                        NtStatus = 0xC000008E
	STATUS_FLOAT_INEXACT_RESULT                                        NtStatus = 0xC000008F
	STATUS_FLOAT_INVALID_OPERATION                                     NtStatus = 0xC0000090
	STATUS_FLOAT_OVERFLOW                                              NtStatus = 0xC0000091
	STATUS_FLOAT_STACK_CHECK                                           NtStatus = 0xC0000092
	STATUS_FLOAT_UNDERFLOW                                             NtStatus = 0xC0000093
	STATUS_INTEGER_DIVIDE_BY_ZERO                                      NtStatus = 0xC0000094
	STATUS_INTEGER_OVERFLOW                                            NtStatus = 0xC0000095
	STATUS_PRIVILEGED_INSTRUCTION                                      NtStatus = 0xC0000096
	STATUS_TOO_MANY_PAGING_FILES                                       NtStatus = 0xC0000097
	STATUS_FILE_INVALID                                                NtStatus = 0xC0000098
	STATUS_ALLOTTED_SPACE_EXCEEDED                                     NtStatus = 0xC0000099
	STATUS_INSUFFICIENT_RESOURCES                                      NtStatus = 0xC000009A
	STATUS_DFS_EXIT_PATH_FOUND                                         NtStatus = 0xC000009B
	STATUS_DEVICE_DATA_ERROR                                           NtStatus = 0xC000009C
	STATUS_DEVICE_NOT_CONNECTED                                        NtStatus = 0xC000009D
	STATUS_FREE_VM_NOT_AT_BASE                                         NtStatus = 0xC000009F
	STATUS_MEMORY_NOT_ALLOCATED                                        NtStatus = 0xC00000A0
	STATUS_WORKING_SET_QUOTA                                           NtStatus = 0xC00000A1
	STATUS_MEDIA_WRITE_PROTECTED                                       NtStatus = 0xC00000A2
	STATUS_DEVICE_NOT_READY                                            NtStatus = 0xC00000A3
	STATUS_INVALID_GROUP_ATTRIBUTES                                    NtStatus = 0xC00000A4
	STATUS_BAD_IMPERSONATION_LEVEL                                     NtStatus = 0xC00000A5
	STATUS_CANT_OPEN_ANONYMOUS                                         NtStatus = 0xC00000A6
	STATUS_BAD_VALIDATION_CLASS                                        NtStatus = 0xC00000A7
	STATUS_BAD_TOKEN_TYPE                                              NtStatus = 0xC00000A8
	STATUS_BAD_MASTER_BOOT_RECORD                                      NtStatus = 0xC00000A9
	STATUS_INSTRUCTION_MISALIGNMENT                                    NtStatus = 0xC00000AA
	STATUS_INSTANCE_NOT_AVAILABLE                                      NtStatus = 0xC00000AB
	STATUS_PIPE_NOT_AVAILABLE                                          NtStatus = 0xC00000AC
	STATUS_INVALID_PIPE_STATE                                          NtStatus = 0xC00000AD
	STATUS_PIPE_BUSY                                                   NtStatus = 0xC00000AE
	STATUS_ILLEGAL_FUNCTION                                            NtStatus = 0xC00000AF
	STATUS_PIPE_DISCONNECTED                                           NtStatus = 0xC00000B0
	STATUS_PIPE_CLOSING                                                NtStatus = 0xC00000B1
	STATUS_PIPE_CONNECTED                                              NtStatus = 0xC00000B2
	STATUS_PIPE_LISTENING                                              NtStatus = 0xC00000B3
	STATUS_INVALID_READ_MODE                                           NtStatus = 0xC00000B4
	STATUS_IO_TIMEOUT                                                  NtStatus = 0xC00000B5
	STATUS_FILE_FORCED_CLOSED                                          NtStatus = 0xC00000B6
	STATUS_PROFILING_NOT_STARTED                                       NtStatus = 0xC00000B7
	STATUS_PROFILING_NOT_STOPPED                                       NtStatus = 0xC00000B8
	STATUS_COULD_NOT_INTERPRET                                         NtStatus = 0xC00000B9
	STATUS_FILE_IS_A_DIRECTORY                                         NtStatus = 0xC00000BA
	STATUS_NOT_SUPPORTED                                               NtStatus = 0xC00000BB
	STATUS_REMOTE_NOT_LISTENING                                        NtStatus = 0xC00000BC
	STATUS_DUPLICATE_NAME                                              NtStatus = 0xC00000BD
	STATUS_BAD_NETWORK_PATH                                            NtStatus = 0xC00000BE
	STATUS_NETWORK_BUSY                                                NtStatus = 0xC00000BF
	STATUS_DEVICE_DOES_NOT_EXIST                                       NtStatus = 0xC00000C0
	STATUS_TOO_MANY_COMMANDS                                           NtStatus = 0xC00000C1
	STATUS_ADAPTER_HARDWARE_ERROR                                      NtStatus = 0xC00000C2
	STATUS_INVALID_NETWORK_RESPONSE                                    NtStatus = 0xC00000C3
	STATUS_UNEXPECTED_NETWORK_ERROR                                    NtStatus = 0xC00000C4
	STATUS_BAD_REMOTE_ADAPTER                                          NtStatus = 0xC00000C5
	STATUS_PRINT_QUEUE_FULL                                            NtStatus = 0xC00000C6
	STATUS_NO_SPOOL_SPACE                                              NtStatus = 0xC00000C7
	STATUS_PRINT_CANCELLED                                             NtStatus = 0xC00000C8
	STATUS_NETWORK_NAME_DELETED                                        NtStatus = 0xC00000C9
	STATUS_NETWORK_ACCESS_DENIED                                       NtStatus = 0xC00000CA
	STATUS_BAD_DEVICE_TYPE                                             NtStatus = 0xC00000CB
	STATUS_BAD_NETWORK_NAME                                            NtStatus = 0xC00000CC
	STATUS_TOO_MANY_NAMES                                              NtStatus = 0xC00000CD
	STATUS_TOO_MANY_SESSIONS                                           NtStatus = 0xC00000CE
	STATUS_SHARING_PAUSED                                              NtStatus = 0xC00000CF
	STATUS_REQUEST_NOT_ACCEPTED                                        NtStatus = 0xC00000D0
	STATUS_REDIRECTOR_PAUSED                                           NtStatus = 0xC00000D1
	STATUS_NET_WRITE_FAULT                                             NtStatus = 0xC00000D2
	STATUS_PROFILING_AT_LIMIT                                          NtStatus = 0xC00000D3
	STATUS_NOT_SAME_DEVICE                                             NtStatus = 0xC00000D4
	STATUS_FILE_RENAMED                                                NtStatus = 0xC00000D5
	STATUS_VIRTUAL_CIRCUIT_CLOSED                                      NtStatus = 0xC00000D6
	STATUS_NO_SECURITY_ON_OBJECT                                       NtStatus = 0xC00000D7
	STATUS_CANT_WAIT                                                   NtStatus = 0xC00000D8
	STATUS_PIPE_EMPTY                                                  NtStatus = 0xC00000D9
	STATUS_CANT_ACCESS_DOMAIN_INFO                                     NtStatus = 0xC00000DA
	STATUS_CANT_TERMINATE_SELF                                         NtStatus = 0xC00000DB
	STATUS_INVALID_SERVER_STATE                                        NtStatus = 0xC00000DC
	STATUS_INVALID_DOMAIN_STATE                                        NtStatus = 0xC00000DD
	STATUS_INVALID_DOMAIN_ROLE                                         NtStatus = 0xC00000DE
	STATUS_NO_SUCH_DOMAIN                                              NtStatus = 0xC00000DF
	STATUS_DOMAIN_EXISTS                                               NtStatus = 0xC00000E0
	STATUS_DOMAIN_LIMIT_EXCEEDED                                       NtStatus = 0xC00000E1
	STATUS_OPLOCK_NOT_GRANTED                                          NtStatus = 0xC00000E2
	STATUS_INVALID_OPLOCK_PROTOCOL                                     NtStatus = 0xC00000E3
	STATUS_INTERNAL_DB_CORRUPTION                                      NtStatus = 0xC00000E4
	STATUS_INTERNAL_ERROR                                              NtStatus = 0xC00000E5
	STATUS_GENERIC_NOT_MAPPED                                          NtStatus = 0xC00000E6
	STATUS_BAD_DESCRIPTOR_FORMAT                                       NtStatus = 0xC00000E7
	STATUS_INVALID_USER_BUFFER                                         NtStatus = 0xC00000E8
	STATUS_UNEXPECTED_IO_ERROR                                         NtStatus = 0xC00000E9
	STATUS_UNEXPECTED_MM_CREATE_ERR                                    NtStatus = 0xC00000EA
	STATUS_UNEXPECTED_MM_MAP_ERROR                                     NtStatus = 0xC00000EB
	STATUS_UNEXPECTED_MM_EXTEND_ERR                                    NtStatus = 0xC00000EC
	STATUS_NOT_LOGON_PROCESS                                           NtStatus = 0xC00000ED
	STATUS_LOGON_SESSION_EXISTS                                        NtStatus = 0xC00000EE
	STATUS_INVALID_PARAMETER_1                                         NtStatus = 0xC00000EF
	STATUS_INVALID_PARAMETER_2                                         NtStatus = 0xC00000F0
	STATUS_INVALID_PARAMETER_3                                         NtStatus = 0xC00000F1
	STATUS_INVALID_PARAMETER_4                                         NtStatus = 0xC00000F2
	STATUS_INVALID_PARAMETER_5                                         NtStatus = 0xC00000F3
	STATUS_INVALID_PARAMETER_6                                         NtStatus = 0xC00000F4
	STATUS_INVALID_PARAMETER_7                                         NtStatus = 0xC00000F5
	STATUS_INVALID_PARAMETER_8                                         NtStatus = 0xC00000F6
	STATUS_INVALID_PARAMETER_9                                         NtStatus = 0xC00000F7
	STATUS_INVALID_PARAMETER_10                                        NtStatus = 0xC00000F8
	STATUS_INVALID_PARAMETER_11                                        NtStatus = 0xC00000F9
	STATUS_INVALID_PARAMETER_12                                        NtStatus = 0xC00000FA
	STATUS_REDIRECTOR_NOT_STARTED                                      NtStatus = 0xC00000FB
	STATUS_REDIRECTOR_STARTED                                          NtStatus = 0xC00000FC
	STATUS_STACK_OVERFLOW                                              NtStatus = 0xC00000FD
	STATUS_NO_SUCH_PACKAGE                                             NtStatus = 0xC00000FE
	STATUS_BAD_FUNCTION_TABLE                                          NtStatus = 0xC00000FF
	STATUS_VARIABLE_NOT_FOUND                                          NtStatus = 0xC0000100
	STATUS_DIRECTORY_NOT_EMPTY                                         NtStatus = 0xC0000101
	STATUS_FILE_CORRUPT_ERROR                                          NtStatus = 0xC0000102
	STATUS_NOT_A_DIRECTORY                                             NtStatus = 0xC0000103
	STATUS_BAD_LOGON_SESSION_STATE                                     NtStatus = 0xC0000104
	STATUS_LOGON_SESSION_COLLISION                                     NtStatus = 0xC0000105
	STATUS_NAME_TOO_LONG                                               NtStatus = 0xC0000106
	STATUS_FILES_OPEN                                                  NtStatus = 0xC0000107
	STATUS_CONNECTION_IN_USE                                           NtStatus = 0xC0000108
	STATUS_MESSAGE_NOT_FOUND                                           NtStatus = 0xC0000109
	STATUS_PROCESS_IS_TERMINATING                                      NtStatus = 0xC000010A
	STATUS_INVALID_LOGON_TYPE                                          NtStatus = 0xC000010B
	STATUS_NO_GUID_TRANSLATION                                         NtStatus = 0xC000010C
	STATUS_CANNOT_IMPERSONATE                                          NtStatus = 0xC000010D
	STATUS_IMAGE_ALREADY_LOADED                                        NtStatus = 0xC000010E
	STATUS_NO_LDT                                                      NtStatus = 0xC0000117
	STATUS_INVALID_LDT_SIZE                                            NtStatus = 0xC0000118
	STATUS_INVALID_LDT_OFFSET                                          NtStatus = 0xC0000119
	STATUS_INVALID_LDT_DESCRIPTOR                                      NtStatus = 0xC000011A
	STATUS_INVALID_IMAGE_NE_FORMAT                                     NtStatus = 0xC000011B
	STATUS_RXACT_INVALID_STATE                                         NtStatus = 0xC000011C
	STATUS_RXACT_COMMIT_FAILURE                                        NtStatus = 0xC000011D
	STATUS_MAPPED_FILE_SIZE_ZERO                                       NtStatus = 0xC000011E
	STATUS_TOO_MANY_OPENED_FILES                                       NtStatus = 0xC000011F
	STATUS_CANCELLED                                                   NtStatus = 0xC0000120
	STATUS_CANNOT_DELETE                                               NtStatus = 0xC0000121
	STATUS_INVALID_COMPUTER_NAME                                       NtStatus = 0xC0000122
	STATUS_FILE_DELETED                                                NtStatus = 0xC0000123
	STATUS_SPECIAL_ACCOUNT                                             NtStatus = 0xC0000124
	STATUS_SPECIAL_GROUP                                               NtStatus = 0xC0000125
	STATUS_SPECIAL_USER                                                NtStatus = 0xC0000126
	STATUS_MEMBERS_PRIMARY_GROUP                                       NtStatus = 0xC0000127
	STATUS_FILE_CLOSED                                                 NtStatus = 0xC0000128
	STATUS_TOO_MANY_THREADS                                            NtStatus = 0xC0000129
	STATUS_THREAD_NOT_IN_PROCESS                                       NtStatus = 0xC000012A
	STATUS_TOKEN_ALREADY_IN_USE                                        NtStatus = 0xC000012B
	STATUS_PAGEFILE_QUOTA_EXCEEDED                                     NtStatus = 0xC000012C
	STATUS_COMMITMENT_LIMIT                                            NtStatus = 0xC000012D
	STATUS_INVALID_IMAGE_LE_FORMAT                                     NtStatus = 0xC000012E
	STATUS_INVALID_IMAGE_NOT_MZ                                        NtStatus = 0xC000012F
	STATUS_INVALID_IMAGE_PROTECT                                       NtStatus = 0xC0000130
	STATUS_INVALID_IMAGE_WIN_16                                        NtStatus = 0xC0000131
	STATUS_LOGON_SERVER_CONFLICT                                       NtStatus = 0xC0000132
	STATUS_TIME_DIFFERENCE_AT_DC                                       NtStatus = 0xC0000133
	STATUS_SYNCHRONIZATION_REQUIRED                                    NtStatus = 0xC0000134
	STATUS_DLL_NOT_FOUND                                               NtStatus = 0xC0000135
	STATUS_OPEN_FAILED                                                 NtStatus = 0xC0000136
	STATUS_IO_PRIVILEGE_FAILED                                         NtStatus = 0xC0000137
	STATUS_ORDINAL_NOT_FOUND                                           NtStatus = 0xC0000138
	STATUS_ENTRYPOINT_NOT_FOUND                                        NtStatus = 0xC0000139
	STATUS_CONTROL_C_EXIT                                              NtStatus = 0xC000013A
	STATUS_LOCAL_DISCONNECT                                            NtStatus = 0xC000013B
	STATUS_REMOTE_DISCONNECT                                           NtStatus = 0xC000013C
	STATUS_REMOTE_RESOURCES                                            NtStatus = 0xC000013D
	STATUS_LINK_FAILED                                                 NtStatus = 0xC000013E
	STATUS_LINK_TIMEOUT                                                NtStatus = 0xC000013F
	STATUS_INVALID_CONNECTION                                          NtStatus = 0xC0000140
	STATUS_INVALID_ADDRESS                                             NtStatus = 0xC0000141
	STATUS_DLL_INIT_FAILED                                             NtStatus = 0xC0000142
	STATUS_MISSING_SYSTEMFILE                                          NtStatus = 0xC0000143
	STATUS_UNHANDLED_EXCEPTION                                         NtStatus = 0xC0000144
	STATUS_APP_INIT_FAILURE                                            NtStatus = 0xC0000145
	STATUS_PAGEFILE_CREATE_FAILED                                      NtStatus = 0xC0000146
	STATUS_NO_PAGEFILE                                                 NtStatus = 0xC0000147
	STATUS_INVALID_LEVEL                                               NtStatus = 0xC0000148
	STATUS_WRONG_PASSWORD_CORE                                         NtStatus = 0xC0000149
	STATUS_ILLEGAL_FLOAT_CONTEXT                                       NtStatus = 0xC000014A
	STATUS_PIPE_BROKEN                                                 NtStatus = 0xC000014B
	STATUS_REGISTRY_CORRUPT                                            NtStatus = 0xC000014C
	STATUS_REGISTRY_IO_FAILED                                          NtStatus = 0xC000014D
	STATUS_NO_EVENT_PAIR                                               NtStatus = 0xC000014E
	STATUS_UNRECOGNIZED_VOLUME                                         NtStatus = 0xC000014F
	STATUS_SERIAL_NO_DEVICE_INITED                                     NtStatus = 0xC0000150
	STATUS_NO_SUCH_ALIAS                                               NtStatus = 0xC0000151
	STATUS_MEMBER_NOT_IN_ALIAS                                         NtStatus = 0xC0000152
	STATUS_MEMBER_IN_ALIAS                                             NtStatus = 0xC0000153
	STATUS_ALIAS_EXISTS                                                NtStatus = 0xC0000154
	STATUS_LOGON_NOT_GRANTED                                           NtStatus = 0xC0000155
	STATUS_TOO_MANY_SECRETS                                            NtStatus = 0xC0000156
	STATUS_SECRET_TOO_LONG                                             NtStatus = 0xC0000157
	STATUS_INTERNAL_DB_ERROR                                           NtStatus = 0xC0000158
	STATUS_FULLSCREEN_MODE                                             NtStatus = 0xC0000159
	STATUS_TOO_MANY_CONTEXT_IDS                                        NtStatus = 0xC000015A
	STATUS_LOGON_TYPE_NOT_GRANTED                                      NtStatus = 0xC000015B
	STATUS_NOT_REGISTRY_FILE                                           NtStatus = 0xC000015C
	STATUS_NT_CROSS_ENCRYPTION_REQUIRED                                NtStatus = 0xC000015D
	STATUS_DOMAIN_CTRLR_CONFIG_ERROR                                   NtStatus = 0xC000015E
	STATUS_FT_MISSING_MEMBER                                           NtStatus = 0xC000015F
	STATUS_ILL_FORMED_SERVICE_ENTRY                                    NtStatus = 0xC0000160
	STATUS_ILLEGAL_CHARACTER                                           NtStatus = 0xC0000161
	STATUS_UNMAPPABLE_CHARACTER                                        NtStatus = 0xC0000162
	STATUS_UNDEFINED_CHARACTER                                         NtStatus = 0xC0000163
	STATUS_FLOPPY_VOLUME                                               NtStatus = 0xC0000164
	STATUS_FLOPPY_ID_MARK_NOT_FOUND                                    NtStatus = 0xC0000165
	STATUS_FLOPPY_WRONG_CYLINDER                                       NtStatus = 0xC0000166
	STATUS_FLOPPY_UNKNOWN_ERROR                                        NtStatus = 0xC0000167
	STATUS_FLOPPY_BAD_REGISTERS                                        NtStatus = 0xC0000168
	STATUS_DISK_RECALIBRATE_FAILED                                     NtStatus = 0xC0000169
	STATUS_DISK_OPERATION_FAILED                                       NtStatus = 0xC000016A
	STATUS_DISK_RESET_FAILED                                           NtStatus = 0xC000016B
	STATUS_SHARED_IRQ_BUSY                                             NtStatus = 0xC000016C
	STATUS_FT_ORPHANING                                                NtStatus = 0xC000016D
	STATUS_BIOS_FAILED_TO_CONNECT_INTERRUPT                            NtStatus = 0xC000016E
	STATUS_PARTITION_FAILURE                                           NtStatus = 0xC0000172
	STATUS_INVALID_BLOCK_LENGTH                                        NtStatus = 0xC0000173
	STATUS_DEVICE_NOT_PARTITIONED                                      NtStatus = 0xC0000174
	STATUS_UNABLE_TO_LOCK_MEDIA                                        NtStatus = 0xC0000175
	STATUS_UNABLE_TO_UNLOAD_MEDIA                                      NtStatus = 0xC0000176
	STATUS_EOM_OVERFLOW                                                NtStatus = 0xC0000177
	STATUS_NO_MEDIA                                                    NtStatus = 0xC0000178
	STATUS_NO_SUCH_MEMBER                                              NtStatus = 0xC000017A
	STATUS_INVALID_MEMBER                                              NtStatus = 0xC000017B
	STATUS_KEY_DELETED                                                 NtStatus = 0xC000017C
	STATUS_NO_LOG_SPACE                                                NtStatus = 0xC000017D
	STATUS_TOO_MANY_SIDS                                               NtStatus = 0xC000017E
	STATUS_LM_CROSS_ENCRYPTION_REQUIRED                                NtStatus = 0xC000017F
	STATUS_KEY_HAS_CHILDREN                                            NtStatus = 0xC0000180
	STATUS_CHILD_MUST_BE_VOLATILE                                      NtStatus = 0xC0000181
	STATUS_DEVICE_CONFIGURATION_ERROR                                  NtStatus = 0xC0000182
	STATUS_DRIVER_INTERNAL_ERROR                                       NtStatus = 0xC0000183
	STATUS_INVALID_DEVICE_STATE                                        NtStatus = 0xC0000184
	STATUS_IO_DEVICE_ERROR                                             NtStatus = 0xC0000185
	STATUS_DEVICE_PROTOCOL_ERROR                                       NtStatus = 0xC0000186
	STATUS_BACKUP_CONTROLLER                                           NtStatus = 0xC0000187
	STATUS_LOG_FILE_FULL                                               NtStatus = 0xC0000188
	STATUS_TOO_LATE                                                    NtStatus = 0xC0000189
	STATUS_NO_TRUST_LSA_SECRET                                         NtStatus = 0xC000018A
	STATUS_NO_TRUST_SAM_ACCOUNT                                        NtStatus = 0xC000018B
	STATUS_TRUSTED_DOMAIN_FAILURE                                      NtStatus = 0xC000018C
	STATUS_TRUSTED_RELATIONSHIP_FAILURE                                NtStatus = 0xC000018D
	STATUS_EVENTLOG_FILE_CORRUPT                                       NtStatus = 0xC000018E
	STATUS_EVENTLOG_CANT_START                                         NtStatus = 0xC000018F
	STATUS_TRUST_FAILURE                                               NtStatus = 0xC0000190
	STATUS_MUTANT_LIMIT_EXCEEDED                                       NtStatus = 0xC0000191
	STATUS_NETLOGON_NOT_STARTED                                        NtStatus = 0xC0000192
	STATUS_ACCOUNT_EXPIRED                                             NtStatus = 0xC0000193
	STATUS_POSSIBLE_DEADLOCK                                           NtStatus = 0xC0000194
	STATUS_NETWORK_CREDENTIAL_CONFLICT                                 NtStatus = 0xC0000195
	STATUS_REMOTE_SESSION_LIMIT                                        NtStatus = 0xC0000196
	STATUS_EVENTLOG_FILE_CHANGED                                       NtStatus = 0xC0000197
	STATUS_NOLOGON_INTERDOMAIN_TRUST_ACCOUNT                           NtStatus = 0xC0000198
	STATUS_NOLOGON_WORKSTATION_TRUST_ACCOUNT                           NtStatus = 0xC0000199
	STATUS_NOLOGON_SERVER_TRUST_ACCOUNT                                NtStatus = 0xC000019A
	STATUS_DOMAIN_TRUST_INCONSISTENT                                   NtStatus = 0xC000019B
	STATUS_FS_DRIVER_REQUIRED                                          NtStatus = 0xC000019C
	STATUS_IMAGE_ALREADY_LOADED_AS_DLL                                 NtStatus = 0xC000019D
	STATUS_INCOMPATIBLE_WITH_GLOBAL_SHORT_NAME_REGISTRY_SETTING        NtStatus = 0xC000019E
	STATUS_SHORT_NAMES_NOT_ENABLED_ON_VOLUME                           NtStatus = 0xC000019F
	STATUS_SECURITY_STREAM_IS_INCONSISTENT                             NtStatus = 0xC00001A0
	STATUS_INVALID_LOCK_RANGE                                          NtStatus = 0xC00001A1
	STATUS_INVALID_ACE_CONDITION                                       NtStatus = 0xC00001A2
	STATUS_IMAGE_SUBSYSTEM_NOT_PRESENT                                 NtStatus = 0xC00001A3
	STATUS_NOTIFICATION_GUID_ALREADY_DEFINED                           NtStatus = 0xC00001A4
	STATUS_NETWORK_OPEN_RESTRICTION                                    NtStatus = 0xC0000201
	STATUS_NO_USER_SESSION_KEY                                         NtStatus = 0xC0000202
	STATUS_USER_SESSION_DELETED                                        NtStatus = 0xC0000203
	STATUS_RESOURCE_LANG_NOT_FOUND                                     NtStatus = 0xC0000204
	STATUS_INSUFF_SERVER_RESOURCES                                     NtStatus = 0xC0000205
	STATUS_INVALID_BUFFER_SIZE                                         NtStatus = 0xC0000206
	STATUS_INVALID_ADDRESS_COMPONENT                                   NtStatus = 0xC0000207
	STATUS_INVALID_ADDRESS_WILDCARD                                    NtStatus = 0xC0000208
	STATUS_TOO_MANY_ADDRESSES                                          NtStatus = 0xC0000209
	STATUS_ADDRESS_ALREADY_EXISTS                                      NtStatus = 0xC000020A
	STATUS_ADDRESS_CLOSED                                              NtStatus = 0xC000020B
	STATUS_CONNECTION_DISCONNECTED                                     NtStatus = 0xC000020C
	STATUS_CONNECTION_RESET                                            NtStatus = 0xC000020D
	STATUS_TOO_MANY_NODES                                              NtStatus = 0xC000020E
	STATUS_TRANSACTION_ABORTED                                         NtStatus = 0xC000020F
	STATUS_TRANSACTION_TIMED_OUT                                       NtStatus = 0xC0000210
	STATUS_TRANSACTION_NO_RELEASE                                      NtStatus = 0xC0000211
	STATUS_TRANSACTION_NO_MATCH                                        NtStatus = 0xC0000212
	STATUS_TRANSACTION_RESPONDED                                       NtStatus = 0xC0000213
	STATUS_TRANSACTION_INVALID_ID                                      NtStatus = 0xC0000214
	STATUS_TRANSACTION_INVALID_TYPE                                    NtStatus = 0xC0000215
	STATUS_NOT_SERVER_SESSION                                          NtStatus = 0xC0000216
	STATUS_NOT_CLIENT_SESSION                                          NtStatus = 0xC0000217
	STATUS_CANNOT_LOAD_REGISTRY_FILE                                   NtStatus = 0xC0000218
	STATUS_DEBUG_ATTACH_FAILED                                         NtStatus = 0xC0000219
	STATUS_SYSTEM_PROCESS_TERMINATED                                   NtStatus = 0xC000021A
	STATUS_DATA_NOT_ACCEPTED                                           NtStatus = 0xC000021B
	STATUS_NO_BROWSER_SERVERS_FOUND                                    NtStatus = 0xC000021C
	STATUS_VDM_HARD_ERROR                                              NtStatus = 0xC000021D
	STATUS_DRIVER_CANCEL_TIMEOUT                                       NtStatus = 0xC000021E
	STATUS_REPLY_MESSAGE_MISMATCH                                      NtStatus = 0xC000021F
	STATUS_MAPPED_ALIGNMENT                                            NtStatus = 0xC0000220
	STATUS_IMAGE_CHECKSUM_MISMATCH                                     NtStatus = 0xC0000221
	STATUS_LOST_WRITEBEHIND_DATA                                       NtStatus = 0xC0000222
	STATUS_CLIENT_SERVER_PARAMETERS_INVALID                            NtStatus = 0xC0000223
	STATUS_PASSWORD_MUST_CHANGE                                        NtStatus = 0xC0000224
	STATUS_NOT_FOUND                                                   NtStatus = 0xC0000225
	STATUS_NOT_TINY_STREAM                                             NtStatus = 0xC0000226
	STATUS_RECOVERY_FAILURE                                            NtStatus = 0xC0000227
	STATUS_STACK_OVERFLOW_READ                                         NtStatus = 0xC0000228
	STATUS_FAIL_CHECK                                                  NtStatus = 0xC0000229
	STATUS_DUPLICATE_OBJECTID                                          NtStatus = 0xC000022A
	STATUS_OBJECTID_EXISTS                                             NtStatus = 0xC000022B
	STATUS_CONVERT_TO_LARGE                                            NtStatus = 0xC000022C
	STATUS_RETRY                                                       NtStatus = 0xC000022D
	STATUS_FOUND_OUT_OF_SCOPE                                          NtStatus = 0xC000022E
	STATUS_ALLOCATE_BUCKET                                             NtStatus = 0xC000022F
	STATUS_PROPSET_NOT_FOUND                                           NtStatus = 0xC0000230
	STATUS_MARSHALL_OVERFLOW                                           NtStatus = 0xC0000231
	STATUS_INVALID_VARIANT                                             NtStatus = 0xC0000232
	STATUS_DOMAIN_CONTROLLER_NOT_FOUND                                 NtStatus = 0xC0000233
	STATUS_ACCOUNT_LOCKED_OUT                                          NtStatus = 0xC0000234
	STATUS_HANDLE_NOT_CLOSABLE                                         NtStatus = 0xC0000235
	STATUS_CONNECTION_REFUSED                                          NtStatus = 0xC0000236
	STATUS_GRACEFUL_DISCONNECT                                         NtStatus = 0xC0000237
	STATUS_ADDRESS_ALREADY_ASSOCIATED                                  NtStatus = 0xC0000238
	STATUS_ADDRESS_NOT_ASSOCIATED                                      NtStatus = 0xC0000239
	STATUS_CONNECTION_INVALID                                          NtStatus = 0xC000023A
	STATUS_CONNECTION_ACTIVE                                           NtStatus = 0xC000023B
	STATUS_NETWORK_UNREACHABLE                                         NtStatus = 0xC000023C
	STATUS_HOST_UNREACHABLE                                            NtStatus = 0xC000023D
	STATUS_PROTOCOL_UNREACHABLE                                        NtStatus = 0xC000023E
	STATUS_PORT_UNREACHABLE                                            NtStatus = 0xC000023F
	STATUS_REQUEST_ABORTED                                             NtStatus = 0xC0000240
	STATUS_CONNECTION_ABORTED                                          NtStatus = 0xC0000241
	STATUS_BAD_COMPRESSION_BUFFER                                      NtStatus = 0xC0000242
	STATUS_USER_MAPPED_FILE                                            NtStatus = 0xC0000243
	STATUS_AUDIT_FAILED                                                NtStatus = 0xC0000244
	STATUS_TIMER_RESOLUTION_NOT_SET                                    NtStatus = 0xC0000245
	STATUS_CONNECTION_COUNT_LIMIT                                      NtStatus = 0xC0000246
	STATUS_LOGIN_TIME_RESTRICTION                                      NtStatus = 0xC0000247
	STATUS_LOGIN_WKSTA_RESTRICTION                                     NtStatus = 0xC0000248
	STATUS_IMAGE_MP_UP_MISMATCH                                        NtStatus = 0xC0000249
	STATUS_INSUFFICIENT_LOGON_INFO                                     NtStatus = 0xC0000250
	STATUS_BAD_DLL_ENTRYPOINT                                          NtStatus = 0xC0000251
	STATUS_BAD_SERVICE_ENTRYPOINT                                      NtStatus = 0xC0000252
	STATUS_LPC_REPLY_LOST                                              NtStatus = 0xC0000253
	STATUS_IP_ADDRESS_CONFLICT1                                        NtStatus = 0xC0000254
	STATUS_IP_ADDRESS_CONFLICT2                                        NtStatus = 0xC0000255
	STATUS_REGISTRY_QUOTA_LIMIT                                        NtStatus = 0xC0000256
	STATUS_PATH_NOT_COVERED                                            NtStatus = 0xC0000257
	STATUS_NO_CALLBACK_ACTIVE                                          NtStatus = 0xC0000258
	STATUS_LICENSE_QUOTA_EXCEEDED                                      NtStatus = 0xC0000259
	STATUS_PWD_TOO_SHORT                                               NtStatus = 0xC000025A
	STATUS_PWD_TOO_RECENT                                              NtStatus = 0xC000025B
	STATUS_PWD_HISTORY_CONFLICT                                        NtStatus = 0xC000025C
	STATUS_PLUGPLAY_NO_DEVICE                                          NtStatus = 0xC000025E
	STATUS_UNSUPPORTED_COMPRESSION                                     NtStatus = 0xC000025F
	STATUS_INVALID_HW_PROFILE                                          NtStatus = 0xC0000260
	STATUS_INVALID_PLUGPLAY_DEVICE_PATH                                NtStatus = 0xC0000261
	STATUS_DRIVER_ORDINAL_NOT_FOUND                                    NtStatus = 0xC0000262
	STATUS_DRIVER_ENTRYPOINT_NOT_FOUND                                 NtStatus = 0xC0000263
	STATUS_RESOURCE_NOT_OWNED                                          NtStatus = 0xC0000264
	STATUS_TOO_MANY_LINKS                                              NtStatus = 0xC0000265
	STATUS_QUOTA_LIST_INCONSISTENT                                     NtStatus = 0xC0000266
	STATUS_FILE_IS_OFFLINE                                             NtStatus = 0xC0000267
	STATUS_EVALUATION_EXPIRATION                                       NtStatus = 0xC0000268
	STATUS_ILLEGAL_DLL_RELOCATION                                      NtStatus = 0xC0000269
	STATUS_LICENSE_VIOLATION                                           NtStatus = 0xC000026A
	STATUS_DLL_INIT_FAILED_LOGOFF                                      NtStatus = 0xC000026B
	STATUS_DRIVER_UNABLE_TO_LOAD                                       NtStatus = 0xC000026C
	STATUS_DFS_UNAVAILABLE                                             NtStatus = 0xC000026D
	STATUS_VOLUME_DISMOUNTED                                           NtStatus = 0xC000026E
	STATUS_WX86_INTERNAL_ERROR                                         NtStatus = 0xC000026F
	STATUS_WX86_FLOAT_STACK_CHECK                                      NtStatus = 0xC0000270
	STATUS_VALIDATE_CONTINUE                                           NtStatus = 0xC0000271
	STATUS_NO_MATCH                                                    NtStatus = 0xC0000272
	STATUS_NO_MORE_MATCHES                                             NtStatus = 0xC0000273
	STATUS_NOT_A_REPARSE_POINT                                         NtStatus = 0xC0000275
	STATUS_IO_REPARSE_TAG_INVALID                                      NtStatus = 0xC0000276
	STATUS_IO_REPARSE_TAG_MISMATCH                                     NtStatus = 0xC0000277
	STATUS_IO_REPARSE_DATA_INVALID                                     NtStatus = 0xC0000278
	STATUS_IO_REPARSE_TAG_NOT_HANDLED                                  NtStatus = 0xC0000279
	STATUS_REPARSE_POINT_NOT_RESOLVED                                  NtStatus = 0xC0000280
	STATUS_DIRECTORY_IS_A_REPARSE_POINT                                NtStatus = 0xC0000281
	STATUS_RANGE_LIST_CONFLICT                                         NtStatus = 0xC0000282
	STATUS_SOURCE_ELEMENT_EMPTY                                        NtStatus = 0xC0000283
	STATUS_DESTINATION_ELEMENT_FULL                                    NtStatus = 0xC0000284
	STATUS_ILLEGAL_ELEMENT_ADDRESS                                     NtStatus = 0xC0000285
	STATUS_MAGAZINE_NOT_PRESENT                                        NtStatus = 0xC0000286
	STATUS_REINITIALIZATION_NEEDED                                     NtStatus = 0xC0000287
	STATUS_ENCRYPTION_FAILED                                           NtStatus = 0xC000028A
	STATUS_DECRYPTION_FAILED                                           NtStatus = 0xC000028B
	STATUS_RANGE_NOT_FOUND                                             NtStatus = 0xC000028C
	STATUS_NO_RECOVERY_POLICY                                          NtStatus = 0xC000028D
	STATUS_NO_EFS                                                      NtStatus = 0xC000028E
	STATUS_WRONG_EFS                                                   NtStatus = 0xC000028F
	STATUS_NO_USER_KEYS                                                NtStatus = 0xC0000290
	STATUS_FILE_NOT_ENCRYPTED                                          NtStatus = 0xC0000291
	STATUS_NOT_EXPORT_FORMAT                                           NtStatus = 0xC0000292
	STATUS_FILE_ENCRYPTED                                              NtStatus = 0xC0000293
	STATUS_WMI_GUID_NOT_FOUND                                          NtStatus = 0xC0000295
	STATUS_WMI_INSTANCE_NOT_FOUND                                      NtStatus = 0xC0000296
	STATUS_WMI_ITEMID_NOT_FOUND                                        NtStatus = 0xC0000297
	STATUS_WMI_TRY_AGAIN                                               NtStatus = 0xC0000298
	STATUS_SHARED_POLICY                                               NtStatus = 0xC0000299
	STATUS_POLICY_OBJECT_NOT_FOUND                                     NtStatus = 0xC000029A
	STATUS_POLICY_ONLY_IN_DS                                           NtStatus = 0xC000029B
	STATUS_VOLUME_NOT_UPGRADED                                         NtStatus = 0xC000029C
	STATUS_REMOTE_STORAGE_NOT_ACTIVE                                   NtStatus = 0xC000029D
	STATUS_REMOTE_STORAGE_MEDIA_ERROR                                  NtStatus = 0xC000029E
	STATUS_NO_TRACKING_SERVICE                                         NtStatus = 0xC000029F
	STATUS_SERVER_SID_MISMATCH                                         NtStatus = 0xC00002A0
	STATUS_DS_NO_ATTRIBUTE_OR_VALUE                                    NtStatus = 0xC00002A1
	STATUS_DS_INVALID_ATTRIBUTE_SYNTAX                                 NtStatus = 0xC00002A2
	STATUS_DS_ATTRIBUTE_TYPE_UNDEFINED                                 NtStatus = 0xC00002A3
	STATUS_DS_ATTRIBUTE_OR_VALUE_EXISTS                                NtStatus = 0xC00002A4
	STATUS_DS_BUSY                                                     NtStatus = 0xC00002A5
	STATUS_DS_UNAVAILABLE                                              NtStatus = 0xC00002A6
	STATUS_DS_NO_RIDS_ALLOCATED                                        NtStatus = 0xC00002A7
	STATUS_DS_NO_MORE_RIDS                                             NtStatus = 0xC00002A8
	STATUS_DS_INCORRECT_ROLE_OWNER                                     NtStatus = 0xC00002A9
	STATUS_DS_RIDMGR_INIT_ERROR                                        NtStatus = 0xC00002AA
	STATUS_DS_OBJ_CLASS_VIOLATION                                      NtStatus = 0xC00002AB
	STATUS_DS_CANT_ON_NON_LEAF                                         NtStatus = 0xC00002AC
	STATUS_DS_CANT_ON_RDN                                              NtStatus = 0xC00002AD
	STATUS_DS_CANT_MOD_OBJ_CLASS                                       NtStatus = 0xC00002AE
	STATUS_DS_CROSS_DOM_MOVE_FAILED                                    NtStatus = 0xC00002AF
	STATUS_DS_GC_NOT_AVAILABLE                                         NtStatus = 0xC00002B0
	STATUS_DIRECTORY_SERVICE_REQUIRED                                  NtStatus = 0xC00002B1
	STATUS_REPARSE_ATTRIBUTE_CONFLICT                                  NtStatus = 0xC00002B2
	STATUS_CANT_ENABLE_DENY_ONLY                                       NtStatus = 0xC00002B3
	STATUS_FLOAT_MULTIPLE_FAULTS                                       NtStatus = 0xC00002B4
	STATUS_FLOAT_MULTIPLE_TRAPS                                        NtStatus = 0xC00002B5
	STATUS_DEVICE_REMOVED                                              NtStatus = 0xC00002B6
	STATUS_JOURNAL_DELETE_IN_PROGRESS                                  NtStatus = 0xC00002B7
	STATUS_JOURNAL_NOT_ACTIVE                                          NtStatus = 0xC00002B8
	STATUS_NOINTERFACE                                                 NtStatus = 0xC00002B9
	STATUS_DS_ADMIN_LIMIT_EXCEEDED                                     NtStatus = 0xC00002C1
	STATUS_DRIVER_FAILED_SLEEP                                         NtStatus = 0xC00002C2
	STATUS_MUTUAL_AUTHENTICATION_FAILED                                NtStatus = 0xC00002C3
	STATUS_CORRUPT_SYSTEM_FILE                                         NtStatus = 0xC00002C4
	STATUS_DATATYPE_MISALIGNMENT_ERROR                                 NtStatus = 0xC00002C5
	STATUS_WMI_READ_ONLY                                               NtStatus = 0xC00002C6
	STATUS_WMI_SET_FAILURE                                             NtStatus = 0xC00002C7
	STATUS_COMMITMENT_MINIMUM                                          NtStatus = 0xC00002C8
	STATUS_REG_NAT_CONSUMPTION                                         NtStatus = 0xC00002C9
	STATUS_TRANSPORT_FULL                                              NtStatus = 0xC00002CA
	STATUS_DS_SAM_INIT_FAILURE                                         NtStatus = 0xC00002CB
	STATUS_ONLY_IF_CONNECTED                                           NtStatus = 0xC00002CC
	STATUS_DS_SENSITIVE_GROUP_VIOLATION                                NtStatus = 0xC00002CD
	STATUS_PNP_RESTART_ENUMERATION                                     NtStatus = 0xC00002CE
	STATUS_JOURNAL_ENTRY_DELETED                                       NtStatus = 0xC00002CF
	STATUS_DS_CANT_MOD_PRIMARYGROUPID                                  NtStatus = 0xC00002D0
	STATUS_SYSTEM_IMAGE_BAD_SIGNATURE                                  NtStatus = 0xC00002D1
	STATUS_PNP_REBOOT_REQUIRED                                         NtStatus = 0xC00002D2
	STATUS_POWER_STATE_INVALID                                         NtStatus = 0xC00002D3
	STATUS_DS_INVALID_GROUP_TYPE                                       NtStatus = 0xC00002D4
	STATUS_DS_NO_NEST_GLOBALGROUP_IN_MIXEDDOMAIN                       NtStatus = 0xC00002D5
	STATUS_DS_NO_NEST_LOCALGROUP_IN_MIXEDDOMAIN                        NtStatus = 0xC00002D6
	STATUS_DS_GLOBAL_CANT_HAVE_LOCAL_MEMBER                            NtStatus = 0xC00002D7
	STATUS_DS_GLOBAL_CANT_HAVE_UNIVERSAL_MEMBER                        NtStatus = 0xC00002D8
	STATUS_DS_UNIVERSAL_CANT_HAVE_LOCAL_MEMBER                         NtStatus = 0xC00002D9
	STATUS_DS_GLOBAL_CANT_HAVE_CROSSDOMAIN_MEMBER                      NtStatus = 0xC00002DA
	STATUS_DS_LOCAL_CANT_HAVE_CROSSDOMAIN_LOCAL_MEMBER                 NtStatus = 0xC00002DB
	STATUS_DS_HAVE_PRIMARY_MEMBERS                                     NtStatus = 0xC00002DC
	STATUS_WMI_NOT_SUPPORTED                                           NtStatus = 0xC00002DD
	STATUS_INSUFFICIENT_POWER                                          NtStatus = 0xC00002DE
	STATUS_SAM_NEED_BOOTKEY_PASSWORD                                   NtStatus = 0xC00002DF
	STATUS_SAM_NEED_BOOTKEY_FLOPPY                                     NtStatus = 0xC00002E0
	STATUS_DS_CANT_START                                               NtStatus = 0xC00002E1
	STATUS_DS_INIT_FAILURE                                             NtStatus = 0xC00002E2
	STATUS_SAM_INIT_FAILURE                                            NtStatus = 0xC00002E3
	STATUS_DS_GC_REQUIRED                                              NtStatus = 0xC00002E4
	STATUS_DS_LOCAL_MEMBER_OF_LOCAL_ONLY                               NtStatus = 0xC00002E5
	STATUS_DS_NO_FPO_IN_UNIVERSAL_GROUPS                               NtStatus = 0xC00002E6
	STATUS_DS_MACHINE_ACCOUNT_QUOTA_EXCEEDED                           NtStatus = 0xC00002E7
	STATUS_CURRENT_DOMAIN_NOT_ALLOWED                                  NtStatus = 0xC00002E9
	STATUS_CANNOT_MAKE                                                 NtStatus = 0xC00002EA
	STATUS_SYSTEM_SHUTDOWN                                             NtStatus = 0xC00002EB
	STATUS_DS_INIT_FAILURE_CONSOLE                                     NtStatus = 0xC00002EC
	STATUS_DS_SAM_INIT_FAILURE_CONSOLE                                 NtStatus = 0xC00002ED
	STATUS_UNFINISHED_CONTEXT_DELETED                                  NtStatus = 0xC00002EE
	STATUS_NO_TGT_REPLY                                                NtStatus = 0xC00002EF
	STATUS_OBJECTID_NOT_FOUND                                          NtStatus = 0xC00002F0
	STATUS_NO_IP_ADDRESSES                                             NtStatus = 0xC00002F1
	STATUS_WRONG_CREDENTIAL_HANDLE                                     NtStatus = 0xC00002F2
	STATUS_CRYPTO_SYSTEM_INVALID                                       NtStatus = 0xC00002F3
	STATUS_MAX_REFERRALS_EXCEEDED                                      NtStatus = 0xC00002F4
	STATUS_MUST_BE_KDC                                                 NtStatus = 0xC00002F5
	STATUS_STRONG_CRYPTO_NOT_SUPPORTED                                 NtStatus = 0xC00002F6
	STATUS_TOO_MANY_PRINCIPALS                                         NtStatus = 0xC00002F7
	STATUS_NO_PA_DATA                                                  NtStatus = 0xC00002F8
	STATUS_PKINIT_NAME_MISMATCH                                        NtStatus = 0xC00002F9
	STATUS_SMARTCARD_LOGON_REQUIRED                                    NtStatus = 0xC00002FA
	STATUS_KDC_INVALID_REQUEST                                         NtStatus = 0xC00002FB
	STATUS_KDC_UNABLE_TO_REFER                                         NtStatus = 0xC00002FC
	STATUS_KDC_UNKNOWN_ETYPE                                           NtStatus = 0xC00002FD
	STATUS_SHUTDOWN_IN_PROGRESS                                        NtStatus = 0xC00002FE
	STATUS_SERVER_SHUTDOWN_IN_PROGRESS                                 NtStatus = 0xC00002FF
	STATUS_NOT_SUPPORTED_ON_SBS                                        NtStatus = 0xC0000300
	STATUS_WMI_GUID_DISCONNECTED                                       NtStatus = 0xC0000301
	STATUS_WMI_ALREADY_DISABLED                                        NtStatus = 0xC0000302
	STATUS_WMI_ALREADY_ENABLED                                         NtStatus = 0xC0000303
	STATUS_MFT_TOO_FRAGMENTED                                          NtStatus = 0xC0000304
	STATUS_COPY_PROTECTION_FAILURE                                     NtStatus = 0xC0000305
	STATUS_CSS_AUTHENTICATION_FAILURE                                  NtStatus = 0xC0000306
	STATUS_CSS_KEY_NOT_PRESENT                                         NtStatus = 0xC0000307
	STATUS_CSS_KEY_NOT_ESTABLISHED                                     NtStatus = 0xC0000308
	STATUS_CSS_SCRAMBLED_SECTOR                                        NtStatus = 0xC0000309
	STATUS_CSS_REGION_MISMATCH                                         NtStatus = 0xC000030A
	STATUS_CSS_RESETS_EXHAUSTED                                        NtStatus = 0xC000030B
	STATUS_PKINIT_FAILURE                                              NtStatus = 0xC0000320
	STATUS_SMARTCARD_SUBSYSTEM_FAILURE                                 NtStatus = 0xC0000321
	STATUS_NO_KERB_KEY                                                 NtStatus = 0xC0000322
	STATUS_HOST_DOWN                                                   NtStatus = 0xC0000350
	STATUS_UNSUPPORTED_PREAUTH                                         NtStatus = 0xC0000351
	STATUS_EFS_ALG_BLOB_TOO_BIG                                        NtStatus = 0xC0000352
	STATUS_PORT_NOT_SET                                                NtStatus = 0xC0000353
	STATUS_DEBUGGER_INACTIVE                                           NtStatus = 0xC0000354
	STATUS_DS_VERSION_CHECK_FAILURE                                    NtStatus = 0xC0000355
	STATUS_AUDITING_DISABLED                                           NtStatus = 0xC0000356
	STATUS_PRENT4_MACHINE_ACCOUNT                                      NtStatus = 0xC0000357
	STATUS_DS_AG_CANT_HAVE_UNIVERSAL_MEMBER                            NtStatus = 0xC0000358
	STATUS_INVALID_IMAGE_WIN_32                                        NtStatus = 0xC0000359
	STATUS_INVALID_IMAGE_WIN_64                                        NtStatus = 0xC000035A
	STATUS_BAD_BINDINGS                                                NtStatus = 0xC000035B
	STATUS_NETWORK_SESSION_EXPIRED                                     NtStatus = 0xC000035C
	STATUS_APPHELP_BLOCK                                               NtStatus = 0xC000035D
	STATUS_ALL_SIDS_FILTERED                                           NtStatus = 0xC000035E
	STATUS_NOT_SAFE_MODE_DRIVER                                        NtStatus = 0xC000035F
	STATUS_ACCESS_DISABLED_BY_POLICY_DEFAULT                           NtStatus = 0xC0000361
	STATUS_ACCESS_DISABLED_BY_POLICY_PATH                              NtStatus = 0xC0000362
	STATUS_ACCESS_DISABLED_BY_POLICY_PUBLISHER                         NtStatus = 0xC0000363
	STATUS_ACCESS_DISABLED_BY_POLICY_OTHER                             NtStatus = 0xC0000364
	STATUS_FAILED_DRIVER_ENTRY                                         NtStatus = 0xC0000365
	STATUS_DEVICE_ENUMERATION_ERROR                                    NtStatus = 0xC0000366
	STATUS_MOUNT_POINT_NOT_RESOLVED                                    NtStatus = 0xC0000368
	STATUS_INVALID_DEVICE_OBJECT_PARAMETER                             NtStatus = 0xC0000369
	STATUS_MCA_OCCURED                                                 NtStatus = 0xC000036A
	STATUS_DRIVER_BLOCKED_CRITICAL                                     NtStatus = 0xC000036B
	STATUS_DRIVER_BLOCKED                                              NtStatus = 0xC000036C
	STATUS_DRIVER_DATABASE_ERROR                                       NtStatus = 0xC000036D
	STATUS_SYSTEM_HIVE_TOO_LARGE                                       NtStatus = 0xC000036E
	STATUS_INVALID_IMPORT_OF_NON_DLL                                   NtStatus = 0xC000036F
	STATUS_NO_SECRETS                                                  NtStatus = 0xC0000371
	STATUS_ACCESS_DISABLED_NO_SAFER_UI_BY_POLICY                       NtStatus = 0xC0000372
	STATUS_FAILED_STACK_SWITCH                                         NtStatus = 0xC0000373
	STATUS_HEAP_CORRUPTION                                             NtStatus = 0xC0000374
	STATUS_SMARTCARD_WRONG_PIN                                         NtStatus = 0xC0000380
	STATUS_SMARTCARD_CARD_BLOCKED                                      NtStatus = 0xC0000381
	STATUS_SMARTCARD_CARD_NOT_AUTHENTICATED                            NtStatus = 0xC0000382
	STATUS_SMARTCARD_NO_CARD                                           NtStatus = 0xC0000383
	STATUS_SMARTCARD_NO_KEY_CONTAINER                                  NtStatus = 0xC0000384
	STATUS_SMARTCARD_NO_CERTIFICATE                                    NtStatus = 0xC0000385
	STATUS_SMARTCARD_NO_KEYSET                                         NtStatus = 0xC0000386
	STATUS_SMARTCARD_IO_ERROR                                          NtStatus = 0xC0000387
	STATUS_DOWNGRADE_DETECTED                                          NtStatus = 0xC0000388
	STATUS_SMARTCARD_CERT_REVOKED                                      NtStatus = 0xC0000389
	STATUS_ISSUING_CA_UNTRUSTED                                        NtStatus = 0xC000038A
	STATUS_REVOCATION_OFFLINE_C                                        NtStatus = 0xC000038B
	STATUS_PKINIT_CLIENT_FAILURE                                       NtStatus = 0xC000038C
	STATUS_SMARTCARD_CERT_EXPIRED                                      NtStatus = 0xC000038D
	STATUS_DRIVER_FAILED_PRIOR_UNLOAD                                  NtStatus = 0xC000038E
	STATUS_SMARTCARD_SILENT_CONTEXT                                    NtStatus = 0xC000038F
	STATUS_PER_USER_TRUST_QUOTA_EXCEEDED                               NtStatus = 0xC0000401
	STATUS_ALL_USER_TRUST_QUOTA_EXCEEDED                               NtStatus = 0xC0000402
	STATUS_USER_DELETE_TRUST_QUOTA_EXCEEDED                            NtStatus = 0xC0000403
	STATUS_DS_NAME_NOT_UNIQUE                                          NtStatus = 0xC0000404
	STATUS_DS_DUPLICATE_ID_FOUND                                       NtStatus = 0xC0000405
	STATUS_DS_GROUP_CONVERSION_ERROR                                   NtStatus = 0xC0000406
	STATUS_VOLSNAP_PREPARE_HIBERNATE                                   NtStatus = 0xC0000407
	STATUS_USER2USER_REQUIRED                                          NtStatus = 0xC0000408
	STATUS_STACK_BUFFER_OVERRUN                                        NtStatus = 0xC0000409
	STATUS_NO_S4U_PROT_SUPPORT                                         NtStatus = 0xC000040A
	STATUS_CROSSREALM_DELEGATION_FAILURE                               NtStatus = 0xC000040B
	STATUS_REVOCATION_OFFLINE_KDC                                      NtStatus = 0xC000040C
	STATUS_ISSUING_CA_UNTRUSTED_KDC                                    NtStatus = 0xC000040D
	STATUS_KDC_CERT_EXPIRED                                            NtStatus = 0xC000040E
	STATUS_KDC_CERT_REVOKED                                            NtStatus = 0xC000040F
	STATUS_PARAMETER_QUOTA_EXCEEDED                                    NtStatus = 0xC0000410
	STATUS_HIBERNATION_FAILURE                                         NtStatus = 0xC0000411
	STATUS_DELAY_LOAD_FAILED                                           NtStatus = 0xC0000412
	STATUS_AUTHENTICATION_FIREWALL_FAILED                              NtStatus = 0xC0000413
	STATUS_VDM_DISALLOWED                                              NtStatus = 0xC0000414
	STATUS_HUNG_DISPLAY_DRIVER_THREAD                                  NtStatus = 0xC0000415
	STATUS_INSUFFICIENT_RESOURCE_FOR_SPECIFIED_SHARED_SECTION_SIZE     NtStatus = 0xC0000416
	STATUS_INVALID_CRUNTIME_PARAMETER                                  NtStatus = 0xC0000417
	STATUS_NTLM_BLOCKED                                                NtStatus = 0xC0000418
	STATUS_DS_SRC_SID_EXISTS_IN_FOREST                                 NtStatus = 0xC0000419
	STATUS_DS_DOMAIN_NAME_EXISTS_IN_FOREST                             NtStatus = 0xC000041A
	STATUS_DS_FLAT_NAME_EXISTS_IN_FOREST                               NtStatus = 0xC000041B
	STATUS_INVALID_USER_PRINCIPAL_NAME                                 NtStatus = 0xC000041C
	STATUS_ASSERTION_FAILURE                                           NtStatus = 0xC0000420
	STATUS_VERIFIER_STOP                                               NtStatus = 0xC0000421
	STATUS_CALLBACK_POP_STACK                                          NtStatus = 0xC0000423
	STATUS_INCOMPATIBLE_DRIVER_BLOCKED                                 NtStatus = 0xC0000424
	STATUS_HIVE_UNLOADED                                               NtStatus = 0xC0000425
	STATUS_COMPRESSION_DISABLED                                        NtStatus = 0xC0000426
	STATUS_FILE_SYSTEM_LIMITATION                                      NtStatus = 0xC0000427
	STATUS_INVALID_IMAGE_HASH                                          NtStatus = 0xC0000428
	STATUS_NOT_CAPABLE                                                 NtStatus = 0xC0000429
	STATUS_REQUEST_OUT_OF_SEQUENCE                                     NtStatus = 0xC000042A
	STATUS_IMPLEMENTATION_LIMIT                                        NtStatus = 0xC000042B
	STATUS_ELEVATION_REQUIRED                                          NtStatus = 0xC000042C
	STATUS_NO_SECURITY_CONTEXT                                         NtStatus = 0xC000042D
	STATUS_PKU2U_CERT_FAILURE                                          NtStatus = 0xC000042E
	STATUS_BEYOND_VDL                                                  NtStatus = 0xC0000432
	STATUS_ENCOUNTERED_WRITE_IN_PROGRESS                               NtStatus = 0xC0000433
	STATUS_PTE_CHANGED                                                 NtStatus = 0xC0000434
	STATUS_PURGE_FAILED                                                NtStatus = 0xC0000435
	STATUS_CRED_REQUIRES_CONFIRMATION                                  NtStatus = 0xC0000440
	STATUS_CS_ENCRYPTION_INVALID_SERVER_RESPONSE                       NtStatus = 0xC0000441
	STATUS_CS_ENCRYPTION_UNSUPPORTED_SERVER                            NtStatus = 0xC0000442
	STATUS_CS_ENCRYPTION_EXISTING_ENCRYPTED_FILE                       NtStatus = 0xC0000443
	STATUS_CS_ENCRYPTION_NEW_ENCRYPTED_FILE                            NtStatus = 0xC0000444
	STATUS_CS_ENCRYPTION_FILE_NOT_CSE                                  NtStatus = 0xC0000445
	STATUS_INVALID_LABEL                                               NtStatus = 0xC0000446
	STATUS_DRIVER_PROCESS_TERMINATED                                   NtStatus = 0xC0000450
	STATUS_AMBIGUOUS_SYSTEM_DEVICE                                     NtStatus = 0xC0000451
	STATUS_SYSTEM_DEVICE_NOT_FOUND                                     NtStatus = 0xC0000452
	STATUS_RESTART_BOOT_APPLICATION                                    NtStatus = 0xC0000453
	STATUS_INSUFFICIENT_NVRAM_RESOURCES                                NtStatus = 0xC0000454
	STATUS_NO_RANGES_PROCESSED                                         NtStatus = 0xC0000460
	STATUS_DEVICE_FEATURE_NOT_SUPPORTED                                NtStatus = 0xC0000463
	STATUS_DEVICE_UNREACHABLE                                          NtStatus = 0xC0000464
	STATUS_INVALID_TOKEN                                               NtStatus = 0xC0000465
	STATUS_SERVER_UNAVAILABLE                                          NtStatus = 0xC0000466
	STATUS_INVALID_TASK_NAME                                           NtStatus = 0xC0000500
	STATUS_INVALID_TASK_INDEX                                          NtStatus = 0xC0000501
	STATUS_THREAD_ALREADY_IN_TASK                                      NtStatus = 0xC0000502
	STATUS_CALLBACK_BYPASS                                             NtStatus = 0xC0000503
	STATUS_FAIL_FAST_EXCEPTION                                         NtStatus = 0xC0000602
	STATUS_IMAGE_CERT_REVOKED                                          NtStatus = 0xC0000603
	STATUS_PORT_CLOSED                                                 NtStatus = 0xC0000700
	STATUS_MESSAGE_LOST                                                NtStatus = 0xC0000701
	STATUS_INVALID_MESSAGE                                             NtStatus = 0xC0000702
	STATUS_REQUEST_CANCELED                                            NtStatus = 0xC0000703
	STATUS_RECURSIVE_DISPATCH                                          NtStatus = 0xC0000704
	STATUS_LPC_RECEIVE_BUFFER_EXPECTED                                 NtStatus = 0xC0000705
	STATUS_LPC_INVALID_CONNECTION_USAGE                                NtStatus = 0xC0000706
	STATUS_LPC_REQUESTS_NOT_ALLOWED                                    NtStatus = 0xC0000707
	STATUS_RESOURCE_IN_USE                                             NtStatus = 0xC0000708
	STATUS_HARDWARE_MEMORY_ERROR                                       NtStatus = 0xC0000709
	STATUS_THREADPOOL_HANDLE_EXCEPTION                                 NtStatus = 0xC000070A
	STATUS_THREADPOOL_SET_EVENT_ON_COMPLETION_FAILED                   NtStatus = 0xC000070B
	STATUS_THREADPOOL_RELEASE_SEMAPHORE_ON_COMPLETION_FAILED           NtStatus = 0xC000070C
	STATUS_THREADPOOL_RELEASE_MUTEX_ON_COMPLETION_FAILED               NtStatus = 0xC000070D
	STATUS_THREADPOOL_FREE_LIBRARY_ON_COMPLETION_FAILED                NtStatus = 0xC000070E
	STATUS_THREADPOOL_RELEASED_DURING_OPERATION                        NtStatus = 0xC000070F
	STATUS_CALLBACK_RETURNED_WHILE_IMPERSONATING                       NtStatus = 0xC0000710
	STATUS_APC_RETURNED_WHILE_IMPERSONATING                            NtStatus = 0xC0000711
	STATUS_PROCESS_IS_PROTECTED                                        NtStatus = 0xC0000712
	STATUS_MCA_EXCEPTION                                               NtStatus = 0xC0000713
	STATUS_CERTIFICATE_MAPPING_NOT_UNIQUE                              NtStatus = 0xC0000714
	STATUS_SYMLINK_CLASS_DISABLED                                      NtStatus = 0xC0000715
	STATUS_INVALID_IDN_NORMALIZATION                                   NtStatus = 0xC0000716
	STATUS_NO_UNICODE_TRANSLATION                                      NtStatus = 0xC0000717
	STATUS_ALREADY_REGISTERED                                          NtStatus = 0xC0000718
	STATUS_CONTEXT_MISMATCH                                            NtStatus = 0xC0000719
	STATUS_PORT_ALREADY_HAS_COMPLETION_LIST                            NtStatus = 0xC000071A
	STATUS_CALLBACK_RETURNED_THREAD_PRIORITY                           NtStatus = 0xC000071B
	STATUS_INVALID_THREAD                                              NtStatus = 0xC000071C
	STATUS_CALLBACK_RETURNED_TRANSACTION                               NtStatus = 0xC000071D
	STATUS_CALLBACK_RETURNED_LDR_LOCK                                  NtStatus = 0xC000071E
	STATUS_CALLBACK_RETURNED_LANG                                      NtStatus = 0xC000071F
	STATUS_CALLBACK_RETURNED_PRI_BACK                                  NtStatus = 0xC0000720
	STATUS_DISK_REPAIR_DISABLED                                        NtStatus = 0xC0000800
	STATUS_DS_DOMAIN_RENAME_IN_PROGRESS                                NtStatus = 0xC0000801
	STATUS_DISK_QUOTA_EXCEEDED                                         NtStatus = 0xC0000802
	STATUS_CONTENT_BLOCKED                                             NtStatus = 0xC0000804
	STATUS_BAD_CLUSTERS                                                NtStatus = 0xC0000805
	STATUS_VOLUME_DIRTY                                                NtStatus = 0xC0000806
	STATUS_FILE_CHECKED_OUT                                            NtStatus = 0xC0000901
	STATUS_CHECKOUT_REQUIRED                                           NtStatus = 0xC0000902
	STATUS_BAD_FILE_TYPE                                               NtStatus = 0xC0000903
	STATUS_FILE_TOO_LARGE                                              NtStatus = 0xC0000904
	STATUS_FORMS_AUTH_REQUIRED                                         NtStatus = 0xC0000905
	STATUS_VIRUS_INFECTED                                              NtStatus = 0xC0000906
	STATUS_VIRUS_DELETED                                               NtStatus = 0xC0000907
	STATUS_BAD_MCFG_TABLE                                              NtStatus = 0xC0000908
	STATUS_CANNOT_BREAK_OPLOCK                                         NtStatus = 0xC0000909
	STATUS_WOW_ASSERTION                                               NtStatus = 0xC0009898
	STATUS_INVALID_SIGNATURE                                           NtStatus = 0xC000A000
	STATUS_HMAC_NOT_SUPPORTED                                          NtStatus = 0xC000A001
	STATUS_IPSEC_QUEUE_OVERFLOW                                        NtStatus = 0xC000A010
	STATUS_ND_QUEUE_OVERFLOW                                           NtStatus = 0xC000A011
	STATUS_HOPLIMIT_EXCEEDED                                           NtStatus = 0xC000A012
	STATUS_PROTOCOL_NOT_SUPPORTED                                      NtStatus = 0xC000A013
	STATUS_LOST_WRITEBEHIND_DATA_NETWORK_DISCONNECTED                  NtStatus = 0xC000A080
	STATUS_LOST_WRITEBEHIND_DATA_NETWORK_SERVER_ERROR                  NtStatus = 0xC000A081
	STATUS_LOST_WRITEBEHIND_DATA_LOCAL_DISK_ERROR                      NtStatus = 0xC000A082
	STATUS_XML_PARSE_ERROR                                             NtStatus = 0xC000A083
	STATUS_XMLDSIG_ERROR                                               NtStatus = 0xC000A084
	STATUS_WRONG_COMPARTMENT                                           NtStatus = 0xC000A085
	STATUS_AUTHIP_FAILURE                                              NtStatus = 0xC000A086
	STATUS_DS_OID_MAPPED_GROUP_CANT_HAVE_MEMBERS                       NtStatus = 0xC000A087
	STATUS_DS_OID_NOT_FOUND                                            NtStatus = 0xC000A088
	STATUS_HASH_NOT_SUPPORTED                                          NtStatus = 0xC000A100
	STATUS_HASH_NOT_PRESENT                                            NtStatus = 0xC000A101
	STATUS_OFFLOAD_READ_FLT_NOT_SUPPORTED                              NtStatus = 0xC000A2A1
	STATUS_OFFLOAD_WRITE_FLT_NOT_SUPPORTED                             NtStatus = 0xC000A2A2
	STATUS_OFFLOAD_READ_FILE_NOT_SUPPORTED                             NtStatus = 0xC000A2A3
	STATUS_OFFLOAD_WRITE_FILE_NOT_SUPPORTED                            NtStatus = 0xC000A2A4
	DBG_NO_STATE_CHANGE                                                NtStatus = 0xC0010001
	DBG_APP_NOT_IDLE                                                   NtStatus = 0xC0010002
	RPC_NT_INVALID_STRING_BINDING                                      NtStatus = 0xC0020001
	RPC_NT_WRONG_KIND_OF_BINDING                                       NtStatus = 0xC0020002
	RPC_NT_INVALID_BINDING                                             NtStatus = 0xC0020003
	RPC_NT_PROTSEQ_NOT_SUPPORTED                                       NtStatus = 0xC0020004
	RPC_NT_INVALID_RPC_PROTSEQ                                         NtStatus = 0xC0020005
	RPC_NT_INVALID_STRING_UUID                                         NtStatus = 0xC0020006
	RPC_NT_INVALID_ENDPOINT_FORMAT                                     NtStatus = 0xC0020007
	RPC_NT_INVALID_NET_ADDR                                            NtStatus = 0xC0020008
	RPC_NT_NO_ENDPOINT_FOUND                                           NtStatus = 0xC0020009
	RPC_NT_INVALID_TIMEOUT                                             NtStatus = 0xC002000A
	RPC_NT_OBJECT_NOT_FOUND                                            NtStatus = 0xC002000B
	RPC_NT_ALREADY_REGISTERED                                          NtStatus = 0xC002000C
	RPC_NT_TYPE_ALREADY_REGISTERED                                     NtStatus = 0xC002000D
	RPC_NT_ALREADY_LISTENING                                           NtStatus = 0xC002000E
	RPC_NT_NO_PROTSEQS_REGISTERED                                      NtStatus = 0xC002000F
	RPC_NT_NOT_LISTENING                                               NtStatus = 0xC0020010
	RPC_NT_UNKNOWN_MGR_TYPE                                            NtStatus = 0xC0020011
	RPC_NT_UNKNOWN_IF                                                  NtStatus = 0xC0020012
	RPC_NT_NO_BINDINGS                                                 NtStatus = 0xC0020013
	RPC_NT_NO_PROTSEQS                                                 NtStatus = 0xC0020014
	RPC_NT_CANT_CREATE_ENDPOINT                                        NtStatus = 0xC0020015
	RPC_NT_OUT_OF_RESOURCES                                            NtStatus = 0xC0020016
	RPC_NT_SERVER_UNAVAILABLE                                          NtStatus = 0xC0020017
	RPC_NT_SERVER_TOO_BUSY                                             NtStatus = 0xC0020018
	RPC_NT_INVALID_NETWORK_OPTIONS                                     NtStatus = 0xC0020019
	RPC_NT_NO_CALL_ACTIVE                                              NtStatus = 0xC002001A
	RPC_NT_CALL_FAILED                                                 NtStatus = 0xC002001B
	RPC_NT_CALL_FAILED_DNE                                             NtStatus = 0xC002001C
	RPC_NT_PROTOCOL_ERROR                                              NtStatus = 0xC002001D
	RPC_NT_UNSUPPORTED_TRANS_SYN                                       NtStatus = 0xC002001F
	RPC_NT_UNSUPPORTED_TYPE                                            NtStatus = 0xC0020021
	RPC_NT_INVALID_TAG                                                 NtStatus = 0xC0020022
	RPC_NT_INVALID_BOUND                                               NtStatus = 0xC0020023
	RPC_NT_NO_ENTRY_NAME                                               NtStatus = 0xC0020024
	RPC_NT_INVALID_NAME_SYNTAX                                         NtStatus = 0xC0020025
	RPC_NT_UNSUPPORTED_NAME_SYNTAX                                     NtStatus = 0xC0020026
	RPC_NT_UUID_NO_ADDRESS                                             NtStatus = 0xC0020028
	RPC_NT_DUPLICATE_ENDPOINT                                          NtStatus = 0xC0020029
	RPC_NT_UNKNOWN_AUTHN_TYPE                                          NtStatus = 0xC002002A
	RPC_NT_MAX_CALLS_TOO_SMALL                                         NtStatus = 0xC002002B
	RPC_NT_STRING_TOO_LONG                                             NtStatus = 0xC002002C
	RPC_NT_PROTSEQ_NOT_FOUND                                           NtStatus = 0xC002002D
	RPC_NT_PROCNUM_OUT_OF_RANGE                                        NtStatus = 0xC002002E
	RPC_NT_BINDING_HAS_NO_AUTH                                         NtStatus = 0xC002002F
	RPC_NT_UNKNOWN_AUTHN_SERVICE                                       NtStatus = 0xC0020030
	RPC_NT_UNKNOWN_AUTHN_LEVEL                                         NtStatus = 0xC0020031
	RPC_NT_INVALID_AUTH_IDENTITY                                       NtStatus = 0xC0020032
	RPC_NT_UNKNOWN_AUTHZ_SERVICE                                       NtStatus = 0xC0020033
	EPT_NT_INVALID_ENTRY                                               NtStatus = 0xC0020034
	EPT_NT_CANT_PERFORM_OP                                             NtStatus = 0xC0020035
	EPT_NT_NOT_REGISTERED                                              NtStatus = 0xC0020036
	RPC_NT_NOTHING_TO_EXPORT                                           NtStatus = 0xC0020037
	RPC_NT_INCOMPLETE_NAME                                             NtStatus = 0xC0020038
	RPC_NT_INVALID_VERS_OPTION                                         NtStatus = 0xC0020039
	RPC_NT_NO_MORE_MEMBERS                                             NtStatus = 0xC002003A
	RPC_NT_NOT_ALL_OBJS_UNEXPORTED                                     NtStatus = 0xC002003B
	RPC_NT_INTERFACE_NOT_FOUND                                         NtStatus = 0xC002003C
	RPC_NT_ENTRY_ALREADY_EXISTS                                        NtStatus = 0xC002003D
	RPC_NT_ENTRY_NOT_FOUND                                             NtStatus = 0xC002003E
	RPC_NT_NAME_SERVICE_UNAVAILABLE                                    NtStatus = 0xC002003F
	RPC_NT_INVALID_NAF_ID                                              NtStatus = 0xC0020040
	RPC_NT_CANNOT_SUPPORT                                              NtStatus = 0xC0020041
	RPC_NT_NO_CONTEXT_AVAILABLE                                        NtStatus = 0xC0020042
	RPC_NT_INTERNAL_ERROR                                              NtStatus = 0xC0020043
	RPC_NT_ZERO_DIVIDE                                                 NtStatus = 0xC0020044
	RPC_NT_ADDRESS_ERROR                                               NtStatus = 0xC0020045
	RPC_NT_FP_DIV_ZERO                                                 NtStatus = 0xC0020046
	RPC_NT_FP_UNDERFLOW                                                NtStatus = 0xC0020047
	RPC_NT_FP_OVERFLOW                                                 NtStatus = 0xC0020048
	RPC_NT_CALL_IN_PROGRESS                                            NtStatus = 0xC0020049
	RPC_NT_NO_MORE_BINDINGS                                            NtStatus = 0xC002004A
	RPC_NT_GROUP_MEMBER_NOT_FOUND                                      NtStatus = 0xC002004B
	EPT_NT_CANT_CREATE                                                 NtStatus = 0xC002004C
	RPC_NT_INVALID_OBJECT                                              NtStatus = 0xC002004D
	RPC_NT_NO_INTERFACES                                               NtStatus = 0xC002004F
	RPC_NT_CALL_CANCELLED                                              NtStatus = 0xC0020050
	RPC_NT_BINDING_INCOMPLETE                                          NtStatus = 0xC0020051
	RPC_NT_COMM_FAILURE                                                NtStatus = 0xC0020052
	RPC_NT_UNSUPPORTED_AUTHN_LEVEL                                     NtStatus = 0xC0020053
	RPC_NT_NO_PRINC_NAME                                               NtStatus = 0xC0020054
	RPC_NT_NOT_RPC_ERROR                                               NtStatus = 0xC0020055
	RPC_NT_SEC_PKG_ERROR                                               NtStatus = 0xC0020057
	RPC_NT_NOT_CANCELLED                                               NtStatus = 0xC0020058
	RPC_NT_INVALID_ASYNC_HANDLE                                        NtStatus = 0xC0020062
	RPC_NT_INVALID_ASYNC_CALL                                          NtStatus = 0xC0020063
	RPC_NT_PROXY_ACCESS_DENIED                                         NtStatus = 0xC0020064
	RPC_NT_NO_MORE_ENTRIES                                             NtStatus = 0xC0030001
	RPC_NT_SS_CHAR_TRANS_OPEN_FAIL                                     NtStatus = 0xC0030002
	RPC_NT_SS_CHAR_TRANS_SHORT_FILE                                    NtStatus = 0xC0030003
	RPC_NT_SS_IN_NULL_CONTEXT                                          NtStatus = 0xC0030004
	RPC_NT_SS_CONTEXT_MISMATCH                                         NtStatus = 0xC0030005
	RPC_NT_SS_CONTEXT_DAMAGED                                          NtStatus = 0xC0030006
	RPC_NT_SS_HANDLES_MISMATCH                                         NtStatus = 0xC0030007
	RPC_NT_SS_CANNOT_GET_CALL_HANDLE                                   NtStatus = 0xC0030008
	RPC_NT_NULL_REF_POINTER                                            NtStatus = 0xC0030009
	RPC_NT_ENUM_VALUE_OUT_OF_RANGE                                     NtStatus = 0xC003000A
	RPC_NT_BYTE_COUNT_TOO_SMALL                                        NtStatus = 0xC003000B
	RPC_NT_BAD_STUB_DATA                                               NtStatus = 0xC003000C
	RPC_NT_INVALID_ES_ACTION                                           NtStatus = 0xC0030059
	RPC_NT_WRONG_ES_VERSION                                            NtStatus = 0xC003005A
	RPC_NT_WRONG_STUB_VERSION                                          NtStatus = 0xC003005B
	RPC_NT_INVALID_PIPE_OBJECT                                         NtStatus = 0xC003005C
	RPC_NT_INVALID_PIPE_OPERATION                                      NtStatus = 0xC003005D
	RPC_NT_WRONG_PIPE_VERSION                                          NtStatus = 0xC003005E
	RPC_NT_PIPE_CLOSED                                                 NtStatus = 0xC003005F
	RPC_NT_PIPE_DISCIPLINE_ERROR                                       NtStatus = 0xC0030060
	RPC_NT_PIPE_EMPTY                                                  NtStatus = 0xC0030061
	STATUS_PNP_BAD_MPS_TABLE                                           NtStatus = 0xC0040035
	STATUS_PNP_TRANSLATION_FAILED                                      NtStatus = 0xC0040036
	STATUS_PNP_IRQ_TRANSLATION_FAILED                                  NtStatus = 0xC0040037
	STATUS_PNP_INVALID_ID                                              NtStatus = 0xC0040038
	STATUS_IO_REISSUE_AS_CACHED                                        NtStatus = 0xC0040039
	STATUS_CTX_WINSTATION_NAME_INVALID                                 NtStatus = 0xC00A0001
	STATUS_CTX_INVALID_PD                                              NtStatus = 0xC00A0002
	STATUS_CTX_PD_NOT_FOUND                                            NtStatus = 0xC00A0003
	STATUS_CTX_CLOSE_PENDING                                           NtStatus = 0xC00A0006
	STATUS_CTX_NO_OUTBUF                                               NtStatus = 0xC00A0007
	STATUS_CTX_MODEM_INF_NOT_FOUND                                     NtStatus = 0xC00A0008
	STATUS_CTX_INVALID_MODEMNAME                                       NtStatus = 0xC00A0009
	STATUS_CTX_RESPONSE_ERROR                                          NtStatus = 0xC00A000A
	STATUS_CTX_MODEM_RESPONSE_TIMEOUT                                  NtStatus = 0xC00A000B
	STATUS_CTX_MODEM_RESPONSE_NO_CARRIER                               NtStatus = 0xC00A000C
	STATUS_CTX_MODEM_RESPONSE_NO_DIALTONE                              NtStatus = 0xC00A000D
	STATUS_CTX_MODEM_RESPONSE_BUSY                                     NtStatus = 0xC00A000E
	STATUS_CTX_MODEM_RESPONSE_VOICE                                    NtStatus = 0xC00A000F
	STATUS_CTX_TD_ERROR                                                NtStatus = 0xC00A0010
	STATUS_CTX_LICENSE_CLIENT_INVALID                                  NtStatus = 0xC00A0012
	STATUS_CTX_LICENSE_NOT_AVAILABLE                                   NtStatus = 0xC00A0013
	STATUS_CTX_LICENSE_EXPIRED                                         NtStatus = 0xC00A0014
	STATUS_CTX_WINSTATION_NOT_FOUND                                    NtStatus = 0xC00A0015
	STATUS_CTX_WINSTATION_NAME_COLLISION                               NtStatus = 0xC00A0016
	STATUS_CTX_WINSTATION_BUSY                                         NtStatus = 0xC00A0017
	STATUS_CTX_BAD_VIDEO_MODE                                          NtStatus = 0xC00A0018
	STATUS_CTX_GRAPHICS_INVALID                                        NtStatus = 0xC00A0022
	STATUS_CTX_NOT_CONSOLE                                             NtStatus = 0xC00A0024
	STATUS_CTX_CLIENT_QUERY_TIMEOUT                                    NtStatus = 0xC00A0026
	STATUS_CTX_CONSOLE_DISCONNECT                                      NtStatus = 0xC00A0027
	STATUS_CTX_CONSOLE_CONNECT                                         NtStatus = 0xC00A0028
	STATUS_CTX_SHADOW_DENIED                                           NtStatus = 0xC00A002A
	STATUS_CTX_WINSTATION_ACCESS_DENIED                                NtStatus = 0xC00A002B
	STATUS_CTX_INVALID_WD                                              NtStatus = 0xC00A002E
	STATUS_CTX_WD_NOT_FOUND                                            NtStatus = 0xC00A002F
	STATUS_CTX_SHADOW_INVALID                                          NtStatus = 0xC00A0030
	STATUS_CTX_SHADOW_DISABLED                                         NtStatus = 0xC00A0031
	STATUS_RDP_PROTOCOL_ERROR                                          NtStatus = 0xC00A0032
	STATUS_CTX_CLIENT_LICENSE_NOT_SET                                  NtStatus = 0xC00A0033
	STATUS_CTX_CLIENT_LICENSE_IN_USE                                   NtStatus = 0xC00A0034
	STATUS_CTX_SHADOW_ENDED_BY_MODE_CHANGE                             NtStatus = 0xC00A0035
	STATUS_CTX_SHADOW_NOT_RUNNING                                      NtStatus = 0xC00A0036
	STATUS_CTX_LOGON_DISABLED                                          NtStatus = 0xC00A0037
	STATUS_CTX_SECURITY_LAYER_ERROR                                    NtStatus = 0xC00A0038
	STATUS_TS_INCOMPATIBLE_SESSIONS                                    NtStatus = 0xC00A0039
	STATUS_MUI_FILE_NOT_FOUND                                          NtStatus = 0xC00B0001
	STATUS_MUI_INVALID_FILE                                            NtStatus = 0xC00B0002
	STATUS_MUI_INVALID_RC_CONFIG                                       NtStatus = 0xC00B0003
	STATUS_MUI_INVALID_LOCALE_NAME                                     NtStatus = 0xC00B0004
	STATUS_MUI_INVALID_ULTIMATEFALLBACK_NAME                           NtStatus = 0xC00B0005
	STATUS_MUI_FILE_NOT_LOADED                                         NtStatus = 0xC00B0006
	STATUS_RESOURCE_ENUM_USER_STOP                                     NtStatus = 0xC00B0007
	STATUS_CLUSTER_INVALID_NODE                                        NtStatus = 0xC0130001
	STATUS_CLUSTER_NODE_EXISTS                                         NtStatus = 0xC0130002
	STATUS_CLUSTER_JOIN_IN_PROGRESS                                    NtStatus = 0xC0130003
	STATUS_CLUSTER_NODE_NOT_FOUND                                      NtStatus = 0xC0130004
	STATUS_CLUSTER_LOCAL_NODE_NOT_FOUND                                NtStatus = 0xC0130005
	STATUS_CLUSTER_NETWORK_EXISTS                                      NtStatus = 0xC0130006
	STATUS_CLUSTER_NETWORK_NOT_FOUND                                   NtStatus = 0xC0130007
	STATUS_CLUSTER_NETINTERFACE_EXISTS                                 NtStatus = 0xC0130008
	STATUS_CLUSTER_NETINTERFACE_NOT_FOUND                              NtStatus = 0xC0130009
	STATUS_CLUSTER_INVALID_REQUEST                                     NtStatus = 0xC013000A
	STATUS_CLUSTER_INVALID_NETWORK_PROVIDER                            NtStatus = 0xC013000B
	STATUS_CLUSTER_NODE_DOWN                                           NtStatus = 0xC013000C
	STATUS_CLUSTER_NODE_UNREACHABLE                                    NtStatus = 0xC013000D
	STATUS_CLUSTER_NODE_NOT_MEMBER                                     NtStatus = 0xC013000E
	STATUS_CLUSTER_JOIN_NOT_IN_PROGRESS                                NtStatus = 0xC013000F
	STATUS_CLUSTER_INVALID_NETWORK                                     NtStatus = 0xC0130010
	STATUS_CLUSTER_NO_NET_ADAPTERS                                     NtStatus = 0xC0130011
	STATUS_CLUSTER_NODE_UP                                             NtStatus = 0xC0130012
	STATUS_CLUSTER_NODE_PAUSED                                         NtStatus = 0xC0130013
	STATUS_CLUSTER_NODE_NOT_PAUSED                                     NtStatus = 0xC0130014
	STATUS_CLUSTER_NO_SECURITY_CONTEXT                                 NtStatus = 0xC0130015
	STATUS_CLUSTER_NETWORK_NOT_INTERNAL                                NtStatus = 0xC0130016
	STATUS_CLUSTER_POISONED                                            NtStatus = 0xC0130017
	STATUS_ACPI_INVALID_OPCODE                                         NtStatus = 0xC0140001
	STATUS_ACPI_STACK_OVERFLOW                                         NtStatus = 0xC0140002
	STATUS_ACPI_ASSERT_FAILED                                          NtStatus = 0xC0140003
	STATUS_ACPI_INVALID_INDEX                                          NtStatus = 0xC0140004
	STATUS_ACPI_INVALID_ARGUMENT                                       NtStatus = 0xC0140005
	STATUS_ACPI_FATAL                                                  NtStatus = 0xC0140006
	STATUS_ACPI_INVALID_SUPERNAME                                      NtStatus = 0xC0140007
	STATUS_ACPI_INVALID_ARGTYPE                                        NtStatus = 0xC0140008
	STATUS_ACPI_INVALID_OBJTYPE                                        NtStatus = 0xC0140009
	STATUS_ACPI_INVALID_TARGETTYPE                                     NtStatus = 0xC014000A
	STATUS_ACPI_INCORRECT_ARGUMENT_COUNT                               NtStatus = 0xC014000B
	STATUS_ACPI_ADDRESS_NOT_MAPPED                                     NtStatus = 0xC014000C
	STATUS_ACPI_INVALID_EVENTTYPE                                      NtStatus = 0xC014000D
	STATUS_ACPI_HANDLER_COLLISION                                      NtStatus = 0xC014000E
	STATUS_ACPI_INVALID_DATA                                           NtStatus = 0xC014000F
	STATUS_ACPI_INVALID_REGION                                         NtStatus = 0xC0140010
	STATUS_ACPI_INVALID_ACCESS_SIZE                                    NtStatus = 0xC0140011
	STATUS_ACPI_ACQUIRE_GLOBAL_LOCK                                    NtStatus = 0xC0140012
	STATUS_ACPI_ALREADY_INITIALIZED                                    NtStatus = 0xC0140013
	STATUS_ACPI_NOT_INITIALIZED                                        NtStatus = 0xC0140014
	STATUS_ACPI_INVALID_MUTEX_LEVEL                                    NtStatus = 0xC0140015
	STATUS_ACPI_MUTEX_NOT_OWNED                                        NtStatus = 0xC0140016
	STATUS_ACPI_MUTEX_NOT_OWNER                                        NtStatus = 0xC0140017
	STATUS_ACPI_RS_ACCESS                                              NtStatus = 0xC0140018
	STATUS_ACPI_INVALID_TABLE                                          NtStatus = 0xC0140019
	STATUS_ACPI_REG_HANDLER_FAILED                                     NtStatus = 0xC0140020
	STATUS_ACPI_POWER_REQUEST_FAILED                                   NtStatus = 0xC0140021
	STATUS_SXS_SECTION_NOT_FOUND                                       NtStatus = 0xC0150001
	STATUS_SXS_CANT_GEN_ACTCTX                                         NtStatus = 0xC0150002
	STATUS_SXS_INVALID_ACTCTXDATA_FORMAT                               NtStatus = 0xC0150003
	STATUS_SXS_ASSEMBLY_NOT_FOUND                                      NtStatus = 0xC0150004
	STATUS_SXS_MANIFEST_FORMAT_ERROR                                   NtStatus = 0xC0150005
	STATUS_SXS_MANIFEST_PARSE_ERROR                                    NtStatus = 0xC0150006
	STATUS_SXS_ACTIVATION_CONTEXT_DISABLED                             NtStatus = 0xC0150007
	STATUS_SXS_KEY_NOT_FOUND                                           NtStatus = 0xC0150008
	STATUS_SXS_VERSION_CONFLICT                                        NtStatus = 0xC0150009
	STATUS_SXS_WRONG_SECTION_TYPE                                      NtStatus = 0xC015000A
	STATUS_SXS_THREAD_QUERIES_DISABLED                                 NtStatus = 0xC015000B
	STATUS_SXS_ASSEMBLY_MISSING                                        NtStatus = 0xC015000C
	STATUS_SXS_PROCESS_DEFAULT_ALREADY_SET                             NtStatus = 0xC015000E
	STATUS_SXS_EARLY_DEACTIVATION                                      NtStatus = 0xC015000F
	STATUS_SXS_INVALID_DEACTIVATION                                    NtStatus = 0xC0150010
	STATUS_SXS_MULTIPLE_DEACTIVATION                                   NtStatus = 0xC0150011
	STATUS_SXS_SYSTEM_DEFAULT_ACTIVATION_CONTEXT_EMPTY                 NtStatus = 0xC0150012
	STATUS_SXS_PROCESS_TERMINATION_REQUESTED                           NtStatus = 0xC0150013
	STATUS_SXS_CORRUPT_ACTIVATION_STACK                                NtStatus = 0xC0150014
	STATUS_SXS_CORRUPTION                                              NtStatus = 0xC0150015
	STATUS_SXS_INVALID_IDENTITY_ATTRIBUTE_VALUE                        NtStatus = 0xC0150016
	STATUS_SXS_INVALID_IDENTITY_ATTRIBUTE_NAME                         NtStatus = 0xC0150017
	STATUS_SXS_IDENTITY_DUPLICATE_ATTRIBUTE                            NtStatus = 0xC0150018
	STATUS_SXS_IDENTITY_PARSE_ERROR                                    NtStatus = 0xC0150019
	STATUS_SXS_COMPONENT_STORE_CORRUPT                                 NtStatus = 0xC015001A
	STATUS_SXS_FILE_HASH_MISMATCH                                      NtStatus = 0xC015001B
	STATUS_SXS_MANIFEST_IDENTITY_SAME_BUT_CONTENTS_DIFFERENT           NtStatus = 0xC015001C
	STATUS_SXS_IDENTITIES_DIFFERENT                                    NtStatus = 0xC015001D
	STATUS_SXS_ASSEMBLY_IS_NOT_A_DEPLOYMENT                            NtStatus = 0xC015001E
	STATUS_SXS_FILE_NOT_PART_OF_ASSEMBLY                               NtStatus = 0xC015001F
	STATUS_ADVANCED_INSTALLER_FAILED                                   NtStatus = 0xC0150020
	STATUS_XML_ENCODING_MISMATCH                                       NtStatus = 0xC0150021
	STATUS_SXS_MANIFEST_TOO_BIG                                        NtStatus = 0xC0150022
	STATUS_SXS_SETTING_NOT_REGISTERED                                  NtStatus = 0xC0150023
	STATUS_SXS_TRANSACTION_CLOSURE_INCOMPLETE                          NtStatus = 0xC0150024
	STATUS_SMI_PRIMITIVE_INSTALLER_FAILED                              NtStatus = 0xC0150025
	STATUS_GENERIC_COMMAND_FAILED                                      NtStatus = 0xC0150026
	STATUS_SXS_FILE_HASH_MISSING                                       NtStatus = 0xC0150027
	STATUS_TRANSACTIONAL_CONFLICT                                      NtStatus = 0xC0190001
	STATUS_INVALID_TRANSACTION                                         NtStatus = 0xC0190002
	STATUS_TRANSACTION_NOT_ACTIVE                                      NtStatus = 0xC0190003
	STATUS_TM_INITIALIZATION_FAILED                                    NtStatus = 0xC0190004
	STATUS_RM_NOT_ACTIVE                                               NtStatus = 0xC0190005
	STATUS_RM_METADATA_CORRUPT                                         NtStatus = 0xC0190006
	STATUS_TRANSACTION_NOT_JOINED                                      NtStatus = 0xC0190007
	STATUS_DIRECTORY_NOT_RM                                            NtStatus = 0xC0190008
	STATUS_TRANSACTIONS_UNSUPPORTED_REMOTE                             NtStatus = 0xC019000A
	STATUS_LOG_RESIZE_INVALID_SIZE                                     NtStatus = 0xC019000B
	STATUS_REMOTE_FILE_VERSION_MISMATCH                                NtStatus = 0xC019000C
	STATUS_CRM_PROTOCOL_ALREADY_EXISTS                                 NtStatus = 0xC019000F
	STATUS_TRANSACTION_PROPAGATION_FAILED                              NtStatus = 0xC0190010
	STATUS_CRM_PROTOCOL_NOT_FOUND                                      NtStatus = 0xC0190011
	STATUS_TRANSACTION_SUPERIOR_EXISTS                                 NtStatus = 0xC0190012
	STATUS_TRANSACTION_REQUEST_NOT_VALID                               NtStatus = 0xC0190013
	STATUS_TRANSACTION_NOT_REQUESTED                                   NtStatus = 0xC0190014
	STATUS_TRANSACTION_ALREADY_ABORTED                                 NtStatus = 0xC0190015
	STATUS_TRANSACTION_ALREADY_COMMITTED                               NtStatus = 0xC0190016
	STATUS_TRANSACTION_INVALID_MARSHALL_BUFFER                         NtStatus = 0xC0190017
	STATUS_CURRENT_TRANSACTION_NOT_VALID                               NtStatus = 0xC0190018
	STATUS_LOG_GROWTH_FAILED                                           NtStatus = 0xC0190019
	STATUS_OBJECT_NO_LONGER_EXISTS                                     NtStatus = 0xC0190021
	STATUS_STREAM_MINIVERSION_NOT_FOUND                                NtStatus = 0xC0190022
	STATUS_STREAM_MINIVERSION_NOT_VALID                                NtStatus = 0xC0190023
	STATUS_MINIVERSION_INACCESSIBLE_FROM_SPECIFIED_TRANSACTION         NtStatus = 0xC0190024
	STATUS_CANT_OPEN_MINIVERSION_WITH_MODIFY_INTENT                    NtStatus = 0xC0190025
	STATUS_CANT_CREATE_MORE_STREAM_MINIVERSIONS                        NtStatus = 0xC0190026
	STATUS_HANDLE_NO_LONGER_VALID                                      NtStatus = 0xC0190028
	STATUS_LOG_CORRUPTION_DETECTED                                     NtStatus = 0xC0190030
	STATUS_RM_DISCONNECTED                                             NtStatus = 0xC0190032
	STATUS_ENLISTMENT_NOT_SUPERIOR                                     NtStatus = 0xC0190033
	STATUS_FILE_IDENTITY_NOT_PERSISTENT                                NtStatus = 0xC0190036
	STATUS_CANT_BREAK_TRANSACTIONAL_DEPENDENCY                         NtStatus = 0xC0190037
	STATUS_CANT_CROSS_RM_BOUNDARY                                      NtStatus = 0xC0190038
	STATUS_TXF_DIR_NOT_EMPTY                                           NtStatus = 0xC0190039
	STATUS_INDOUBT_TRANSACTIONS_EXIST                                  NtStatus = 0xC019003A
	STATUS_TM_VOLATILE                                                 NtStatus = 0xC019003B
	STATUS_ROLLBACK_TIMER_EXPIRED                                      NtStatus = 0xC019003C
	STATUS_TXF_ATTRIBUTE_CORRUPT                                       NtStatus = 0xC019003D
	STATUS_EFS_NOT_ALLOWED_IN_TRANSACTION                              NtStatus = 0xC019003E
	STATUS_TRANSACTIONAL_OPEN_NOT_ALLOWED                              NtStatus = 0xC019003F
	STATUS_TRANSACTED_MAPPING_UNSUPPORTED_REMOTE                       NtStatus = 0xC0190040
	STATUS_TRANSACTION_REQUIRED_PROMOTION                              NtStatus = 0xC0190043
	STATUS_CANNOT_EXECUTE_FILE_IN_TRANSACTION                          NtStatus = 0xC0190044
	STATUS_TRANSACTIONS_NOT_FROZEN                                     NtStatus = 0xC0190045
	STATUS_TRANSACTION_FREEZE_IN_PROGRESS                              NtStatus = 0xC0190046
	STATUS_NOT_SNAPSHOT_VOLUME                                         NtStatus = 0xC0190047
	STATUS_NO_SAVEPOINT_WITH_OPEN_FILES                                NtStatus = 0xC0190048
	STATUS_SPARSE_NOT_ALLOWED_IN_TRANSACTION                           NtStatus = 0xC0190049
	STATUS_TM_IDENTITY_MISMATCH                                        NtStatus = 0xC019004A
	STATUS_FLOATED_SECTION                                             NtStatus = 0xC019004B
	STATUS_CANNOT_ACCEPT_TRANSACTED_WORK                               NtStatus = 0xC019004C
	STATUS_CANNOT_ABORT_TRANSACTIONS                                   NtStatus = 0xC019004D
	STATUS_TRANSACTION_NOT_FOUND                                       NtStatus = 0xC019004E
	STATUS_RESOURCEMANAGER_NOT_FOUND                                   NtStatus = 0xC019004F
	STATUS_ENLISTMENT_NOT_FOUND                                        NtStatus = 0xC0190050
	STATUS_TRANSACTIONMANAGER_NOT_FOUND                                NtStatus = 0xC0190051
	STATUS_TRANSACTIONMANAGER_NOT_ONLINE                               NtStatus = 0xC0190052
	STATUS_TRANSACTIONMANAGER_RECOVERY_NAME_COLLISION                  NtStatus = 0xC0190053
	STATUS_TRANSACTION_NOT_ROOT                                        NtStatus = 0xC0190054
	STATUS_TRANSACTION_OBJECT_EXPIRED                                  NtStatus = 0xC0190055
	STATUS_COMPRESSION_NOT_ALLOWED_IN_TRANSACTION                      NtStatus = 0xC0190056
	STATUS_TRANSACTION_RESPONSE_NOT_ENLISTED                           NtStatus = 0xC0190057
	STATUS_TRANSACTION_RECORD_TOO_LONG                                 NtStatus = 0xC0190058
	STATUS_NO_LINK_TRACKING_IN_TRANSACTION                             NtStatus = 0xC0190059
	STATUS_OPERATION_NOT_SUPPORTED_IN_TRANSACTION                      NtStatus = 0xC019005A
	STATUS_TRANSACTION_INTEGRITY_VIOLATED                              NtStatus = 0xC019005B
	STATUS_EXPIRED_HANDLE                                              NtStatus = 0xC0190060
	STATUS_TRANSACTION_NOT_ENLISTED                                    NtStatus = 0xC0190061
	STATUS_LOG_SECTOR_INVALID                                          NtStatus = 0xC01A0001
	STATUS_LOG_SECTOR_PARITY_INVALID                                   NtStatus = 0xC01A0002
	STATUS_LOG_SECTOR_REMAPPED                                         NtStatus = 0xC01A0003
	STATUS_LOG_BLOCK_INCOMPLETE                                        NtStatus = 0xC01A0004
	STATUS_LOG_INVALID_RANGE                                           NtStatus = 0xC01A0005
	STATUS_LOG_BLOCKS_EXHAUSTED                                        NtStatus = 0xC01A0006
	STATUS_LOG_READ_CONTEXT_INVALID                                    NtStatus = 0xC01A0007
	STATUS_LOG_RESTART_INVALID                                         NtStatus = 0xC01A0008
	STATUS_LOG_BLOCK_VERSION                                           NtStatus = 0xC01A0009
	STATUS_LOG_BLOCK_INVALID                                           NtStatus = 0xC01A000A
	STATUS_LOG_READ_MODE_INVALID                                       NtStatus = 0xC01A000B
	STATUS_LOG_METADATA_CORRUPT                                        NtStatus = 0xC01A000D
	STATUS_LOG_METADATA_INVALID                                        NtStatus = 0xC01A000E
	STATUS_LOG_METADATA_INCONSISTENT                                   NtStatus = 0xC01A000F
	STATUS_LOG_RESERVATION_INVALID                                     NtStatus = 0xC01A0010
	STATUS_LOG_CANT_DELETE                                             NtStatus = 0xC01A0011
	STATUS_LOG_CONTAINER_LIMIT_EXCEEDED                                NtStatus = 0xC01A0012
	STATUS_LOG_START_OF_LOG                                            NtStatus = 0xC01A0013
	STATUS_LOG_POLICY_ALREADY_INSTALLED                                NtStatus = 0xC01A0014
	STATUS_LOG_POLICY_NOT_INSTALLED                                    NtStatus = 0xC01A0015
	STATUS_LOG_POLICY_INVALID                                          NtStatus = 0xC01A0016
	STATUS_LOG_POLICY_CONFLICT                                         NtStatus = 0xC01A0017
	STATUS_LOG_PINNED_ARCHIVE_TAIL                                     NtStatus = 0xC01A0018
	STATUS_LOG_RECORD_NONEXISTENT                                      NtStatus = 0xC01A0019
	STATUS_LOG_RECORDS_RESERVED_INVALID                                NtStatus = 0xC01A001A
	STATUS_LOG_SPACE_RESERVED_INVALID                                  NtStatus = 0xC01A001B
	STATUS_LOG_TAIL_INVALID                                            NtStatus = 0xC01A001C
	STATUS_LOG_FULL                                                    NtStatus = 0xC01A001D
	STATUS_LOG_MULTIPLEXED                                             NtStatus = 0xC01A001E
	STATUS_LOG_DEDICATED                                               NtStatus = 0xC01A001F
	STATUS_LOG_ARCHIVE_NOT_IN_PROGRESS                                 NtStatus = 0xC01A0020
	STATUS_LOG_ARCHIVE_IN_PROGRESS                                     NtStatus = 0xC01A0021
	STATUS_LOG_EPHEMERAL                                               NtStatus = 0xC01A0022
	STATUS_LOG_NOT_ENOUGH_CONTAINERS                                   NtStatus = 0xC01A0023
	STATUS_LOG_CLIENT_ALREADY_REGISTERED                               NtStatus = 0xC01A0024
	STATUS_LOG_CLIENT_NOT_REGISTERED                                   NtStatus = 0xC01A0025
	STATUS_LOG_FULL_HANDLER_IN_PROGRESS                                NtStatus = 0xC01A0026
	STATUS_LOG_CONTAINER_READ_FAILED                                   NtStatus = 0xC01A0027
	STATUS_LOG_CONTAINER_WRITE_FAILED                                  NtStatus = 0xC01A0028
	STATUS_LOG_CONTAINER_OPEN_FAILED                                   NtStatus = 0xC01A0029
	STATUS_LOG_CONTAINER_STATE_INVALID                                 NtStatus = 0xC01A002A
	STATUS_LOG_STATE_INVALID                                           NtStatus = 0xC01A002B
	STATUS_LOG_PINNED                                                  NtStatus = 0xC01A002C
	STATUS_LOG_METADATA_FLUSH_FAILED                                   NtStatus = 0xC01A002D
	STATUS_LOG_INCONSISTENT_SECURITY                                   NtStatus = 0xC01A002E
	STATUS_LOG_APPENDED_FLUSH_FAILED                                   NtStatus = 0xC01A002F
	STATUS_LOG_PINNED_RESERVATION                                      NtStatus = 0xC01A0030
	STATUS_VIDEO_HUNG_DISPLAY_DRIVER_THREAD                            NtStatus = 0xC01B00EA
	STATUS_FLT_NO_HANDLER_DEFINED                                      NtStatus = 0xC01C0001
	STATUS_FLT_CONTEXT_ALREADY_DEFINED                                 NtStatus = 0xC01C0002
	STATUS_FLT_INVALID_ASYNCHRONOUS_REQUEST                            NtStatus = 0xC01C0003
	STATUS_FLT_DISALLOW_FAST_IO                                        NtStatus = 0xC01C0004
	STATUS_FLT_INVALID_NAME_REQUEST                                    NtStatus = 0xC01C0005
	STATUS_FLT_NOT_SAFE_TO_POST_OPERATION                              NtStatus = 0xC01C0006
	STATUS_FLT_NOT_INITIALIZED                                         NtStatus = 0xC01C0007
	STATUS_FLT_FILTER_NOT_READY                                        NtStatus = 0xC01C0008
	STATUS_FLT_POST_OPERATION_CLEANUP                                  NtStatus = 0xC01C0009
	STATUS_FLT_INTERNAL_ERROR                                          NtStatus = 0xC01C000A
	STATUS_FLT_DELETING_OBJECT                                         NtStatus = 0xC01C000B
	STATUS_FLT_MUST_BE_NONPAGED_POOL                                   NtStatus = 0xC01C000C
	STATUS_FLT_DUPLICATE_ENTRY                                         NtStatus = 0xC01C000D
	STATUS_FLT_CBDQ_DISABLED                                           NtStatus = 0xC01C000E
	STATUS_FLT_DO_NOT_ATTACH                                           NtStatus = 0xC01C000F
	STATUS_FLT_DO_NOT_DETACH                                           NtStatus = 0xC01C0010
	STATUS_FLT_INSTANCE_ALTITUDE_COLLISION                             NtStatus = 0xC01C0011
	STATUS_FLT_INSTANCE_NAME_COLLISION                                 NtStatus = 0xC01C0012
	STATUS_FLT_FILTER_NOT_FOUND                                        NtStatus = 0xC01C0013
	STATUS_FLT_VOLUME_NOT_FOUND                                        NtStatus = 0xC01C0014
	STATUS_FLT_INSTANCE_NOT_FOUND                                      NtStatus = 0xC01C0015
	STATUS_FLT_CONTEXT_ALLOCATION_NOT_FOUND                            NtStatus = 0xC01C0016
	STATUS_FLT_INVALID_CONTEXT_REGISTRATION                            NtStatus = 0xC01C0017
	STATUS_FLT_NAME_CACHE_MISS                                         NtStatus = 0xC01C0018
	STATUS_FLT_NO_DEVICE_OBJECT                                        NtStatus = 0xC01C0019
	STATUS_FLT_VOLUME_ALREADY_MOUNTED                                  NtStatus = 0xC01C001A
	STATUS_FLT_ALREADY_ENLISTED                                        NtStatus = 0xC01C001B
	STATUS_FLT_CONTEXT_ALREADY_LINKED                                  NtStatus = 0xC01C001C
	STATUS_FLT_NO_WAITER_FOR_REPLY                                     NtStatus = 0xC01C0020
	STATUS_MONITOR_NO_DESCRIPTOR                                       NtStatus = 0xC01D0001
	STATUS_MONITOR_UNKNOWN_DESCRIPTOR_FORMAT                           NtStatus = 0xC01D0002
	STATUS_MONITOR_INVALID_DESCRIPTOR_CHECKSUM                         NtStatus = 0xC01D0003
	STATUS_MONITOR_INVALID_STANDARD_TIMING_BLOCK                       NtStatus = 0xC01D0004
	STATUS_MONITOR_WMI_DATABLOCK_REGISTRATION_FAILED                   NtStatus = 0xC01D0005
	STATUS_MONITOR_INVALID_SERIAL_NUMBER_MONDSC_BLOCK                  NtStatus = 0xC01D0006
	STATUS_MONITOR_INVALID_USER_FRIENDLY_MONDSC_BLOCK                  NtStatus = 0xC01D0007
	STATUS_MONITOR_NO_MORE_DESCRIPTOR_DATA                             NtStatus = 0xC01D0008
	STATUS_MONITOR_INVALID_DETAILED_TIMING_BLOCK                       NtStatus = 0xC01D0009
	STATUS_MONITOR_INVALID_MANUFACTURE_DATE                            NtStatus = 0xC01D000A
	STATUS_GRAPHICS_NOT_EXCLUSIVE_MODE_OWNER                           NtStatus = 0xC01E0000
	STATUS_GRAPHICS_INSUFFICIENT_DMA_BUFFER                            NtStatus = 0xC01E0001
	STATUS_GRAPHICS_INVALID_DISPLAY_ADAPTER                            NtStatus = 0xC01E0002
	STATUS_GRAPHICS_ADAPTER_WAS_RESET                                  NtStatus = 0xC01E0003
	STATUS_GRAPHICS_INVALID_DRIVER_MODEL                               NtStatus = 0xC01E0004
	STATUS_GRAPHICS_PRESENT_MODE_CHANGED                               NtStatus = 0xC01E0005
	STATUS_GRAPHICS_PRESENT_OCCLUDED                                   NtStatus = 0xC01E0006
	STATUS_GRAPHICS_PRESENT_DENIED                                     NtStatus = 0xC01E0007
	STATUS_GRAPHICS_CANNOTCOLORCONVERT                                 NtStatus = 0xC01E0008
	STATUS_GRAPHICS_PRESENT_REDIRECTION_DISABLED                       NtStatus = 0xC01E000B
	STATUS_GRAPHICS_PRESENT_UNOCCLUDED                                 NtStatus = 0xC01E000C
	STATUS_GRAPHICS_NO_VIDEO_MEMORY                                    NtStatus = 0xC01E0100
	STATUS_GRAPHICS_CANT_LOCK_MEMORY                                   NtStatus = 0xC01E0101
	STATUS_GRAPHICS_ALLOCATION_BUSY                                    NtStatus = 0xC01E0102
	STATUS_GRAPHICS_TOO_MANY_REFERENCES                                NtStatus = 0xC01E0103
	STATUS_GRAPHICS_TRY_AGAIN_LATER                                    NtStatus = 0xC01E0104
	STATUS_GRAPHICS_TRY_AGAIN_NOW                                      NtStatus = 0xC01E0105
	STATUS_GRAPHICS_ALLOCATION_INVALID                                 NtStatus = 0xC01E0106
	STATUS_GRAPHICS_UNSWIZZLING_APERTURE_UNAVAILABLE                   NtStatus = 0xC01E0107
	STATUS_GRAPHICS_UNSWIZZLING_APERTURE_UNSUPPORTED                   NtStatus = 0xC01E0108
	STATUS_GRAPHICS_CANT_EVICT_PINNED_ALLOCATION                       NtStatus = 0xC01E0109
	STATUS_GRAPHICS_INVALID_ALLOCATION_USAGE                           NtStatus = 0xC01E0110
	STATUS_GRAPHICS_CANT_RENDER_LOCKED_ALLOCATION                      NtStatus = 0xC01E0111
	STATUS_GRAPHICS_ALLOCATION_CLOSED                                  NtStatus = 0xC01E0112
	STATUS_GRAPHICS_INVALID_ALLOCATION_INSTANCE                        NtStatus = 0xC01E0113
	STATUS_GRAPHICS_INVALID_ALLOCATION_HANDLE                          NtStatus = 0xC01E0114
	STATUS_GRAPHICS_WRONG_ALLOCATION_DEVICE                            NtStatus = 0xC01E0115
	STATUS_GRAPHICS_ALLOCATION_CONTENT_LOST                            NtStatus = 0xC01E0116
	STATUS_GRAPHICS_GPU_EXCEPTION_ON_DEVICE                            NtStatus = 0xC01E0200
	STATUS_GRAPHICS_INVALID_VIDPN_TOPOLOGY                             NtStatus = 0xC01E0300
	STATUS_GRAPHICS_VIDPN_TOPOLOGY_NOT_SUPPORTED                       NtStatus = 0xC01E0301
	STATUS_GRAPHICS_VIDPN_TOPOLOGY_CURRENTLY_NOT_SUPPORTED             NtStatus = 0xC01E0302
	STATUS_GRAPHICS_INVALID_VIDPN                                      NtStatus = 0xC01E0303
	STATUS_GRAPHICS_INVALID_VIDEO_PRESENT_SOURCE                       NtStatus = 0xC01E0304
	STATUS_GRAPHICS_INVALID_VIDEO_PRESENT_TARGET                       NtStatus = 0xC01E0305
	STATUS_GRAPHICS_VIDPN_MODALITY_NOT_SUPPORTED                       NtStatus = 0xC01E0306
	STATUS_GRAPHICS_INVALID_VIDPN_SOURCEMODESET                        NtStatus = 0xC01E0308
	STATUS_GRAPHICS_INVALID_VIDPN_TARGETMODESET                        NtStatus = 0xC01E0309
	STATUS_GRAPHICS_INVALID_FREQUENCY                                  NtStatus = 0xC01E030A
	STATUS_GRAPHICS_INVALID_ACTIVE_REGION                              NtStatus = 0xC01E030B
	STATUS_GRAPHICS_INVALID_TOTAL_REGION                               NtStatus = 0xC01E030C
	STATUS_GRAPHICS_INVALID_VIDEO_PRESENT_SOURCE_MODE                  NtStatus = 0xC01E0310
	STATUS_GRAPHICS_INVALID_VIDEO_PRESENT_TARGET_MODE                  NtStatus = 0xC01E0311
	STATUS_GRAPHICS_PINNED_MODE_MUST_REMAIN_IN_SET                     NtStatus = 0xC01E0312
	STATUS_GRAPHICS_PATH_ALREADY_IN_TOPOLOGY                           NtStatus = 0xC01E0313
	STATUS_GRAPHICS_MODE_ALREADY_IN_MODESET                            NtStatus = 0xC01E0314
	STATUS_GRAPHICS_INVALID_VIDEOPRESENTSOURCESET                      NtStatus = 0xC01E0315
	STATUS_GRAPHICS_INVALID_VIDEOPRESENTTARGETSET                      NtStatus = 0xC01E0316
	STATUS_GRAPHICS_SOURCE_ALREADY_IN_SET                              NtStatus = 0xC01E0317
	STATUS_GRAPHICS_TARGET_ALREADY_IN_SET                              NtStatus = 0xC01E0318
	STATUS_GRAPHICS_INVALID_VIDPN_PRESENT_PATH                         NtStatus = 0xC01E0319
	STATUS_GRAPHICS_NO_RECOMMENDED_VIDPN_TOPOLOGY                      NtStatus = 0xC01E031A
	STATUS_GRAPHICS_INVALID_MONITOR_FREQUENCYRANGESET                  NtStatus = 0xC01E031B
	STATUS_GRAPHICS_INVALID_MONITOR_FREQUENCYRANGE                     NtStatus = 0xC01E031C
	STATUS_GRAPHICS_FREQUENCYRANGE_NOT_IN_SET                          NtStatus = 0xC01E031D
	STATUS_GRAPHICS_FREQUENCYRANGE_ALREADY_IN_SET                      NtStatus = 0xC01E031F
	STATUS_GRAPHICS_STALE_MODESET                                      NtStatus = 0xC01E0320
	STATUS_GRAPHICS_INVALID_MONITOR_SOURCEMODESET                      NtStatus = 0xC01E0321
	STATUS_GRAPHICS_INVALID_MONITOR_SOURCE_MODE                        NtStatus = 0xC01E0322
	STATUS_GRAPHICS_NO_RECOMMENDED_FUNCTIONAL_VIDPN                    NtStatus = 0xC01E0323
	STATUS_GRAPHICS_MODE_ID_MUST_BE_UNIQUE                             NtStatus = 0xC01E0324
	STATUS_GRAPHICS_EMPTY_ADAPTER_MONITOR_MODE_SUPPORT_INTERSECTION    NtStatus = 0xC01E0325
	STATUS_GRAPHICS_VIDEO_PRESENT_TARGETS_LESS_THAN_SOURCES            NtStatus = 0xC01E0326
	STATUS_GRAPHICS_PATH_NOT_IN_TOPOLOGY                               NtStatus = 0xC01E0327
	STATUS_GRAPHICS_ADAPTER_MUST_HAVE_AT_LEAST_ONE_SOURCE              NtStatus = 0xC01E0328
	STATUS_GRAPHICS_ADAPTER_MUST_HAVE_AT_LEAST_ONE_TARGET              NtStatus = 0xC01E0329
	STATUS_GRAPHICS_INVALID_MONITORDESCRIPTORSET                       NtStatus = 0xC01E032A
	STATUS_GRAPHICS_INVALID_MONITORDESCRIPTOR                          NtStatus = 0xC01E032B
	STATUS_GRAPHICS_MONITORDESCRIPTOR_NOT_IN_SET                       NtStatus = 0xC01E032C
	STATUS_GRAPHICS_MONITORDESCRIPTOR_ALREADY_IN_SET                   NtStatus = 0xC01E032D
	STATUS_GRAPHICS_MONITORDESCRIPTOR_ID_MUST_BE_UNIQUE                NtStatus = 0xC01E032E
	STATUS_GRAPHICS_INVALID_VIDPN_TARGET_SUBSET_TYPE                   NtStatus = 0xC01E032F
	STATUS_GRAPHICS_RESOURCES_NOT_RELATED                              NtStatus = 0xC01E0330
	STATUS_GRAPHICS_SOURCE_ID_MUST_BE_UNIQUE                           NtStatus = 0xC01E0331
	STATUS_GRAPHICS_TARGET_ID_MUST_BE_UNIQUE                           NtStatus = 0xC01E0332
	STATUS_GRAPHICS_NO_AVAILABLE_VIDPN_TARGET                          NtStatus = 0xC01E0333
	STATUS_GRAPHICS_MONITOR_COULD_NOT_BE_ASSOCIATED_WITH_ADAPTER       NtStatus = 0xC01E0334
	STATUS_GRAPHICS_NO_VIDPNMGR                                        NtStatus = 0xC01E0335
	STATUS_GRAPHICS_NO_ACTIVE_VIDPN                                    NtStatus = 0xC01E0336
	STATUS_GRAPHICS_STALE_VIDPN_TOPOLOGY                               NtStatus = 0xC01E0337
	STATUS_GRAPHICS_MONITOR_NOT_CONNECTED                              NtStatus = 0xC01E0338
	STATUS_GRAPHICS_SOURCE_NOT_IN_TOPOLOGY                             NtStatus = 0xC01E0339
	STATUS_GRAPHICS_INVALID_PRIMARYSURFACE_SIZE                        NtStatus = 0xC01E033A
	STATUS_GRAPHICS_INVALID_VISIBLEREGION_SIZE                         NtStatus = 0xC01E033B
	STATUS_GRAPHICS_INVALID_STRIDE                                     NtStatus = 0xC01E033C
	STATUS_GRAPHICS_INVALID_PIXELFORMAT                                NtStatus = 0xC01E033D
	STATUS_GRAPHICS_INVALID_COLORBASIS                                 NtStatus = 0xC01E033E
	STATUS_GRAPHICS_INVALID_PIXELVALUEACCESSMODE                       NtStatus = 0xC01E033F
	STATUS_GRAPHICS_TARGET_NOT_IN_TOPOLOGY                             NtStatus = 0xC01E0340
	STATUS_GRAPHICS_NO_DISPLAY_MODE_MANAGEMENT_SUPPORT                 NtStatus = 0xC01E0341
	STATUS_GRAPHICS_VIDPN_SOURCE_IN_USE                                NtStatus = 0xC01E0342
	STATUS_GRAPHICS_CANT_ACCESS_ACTIVE_VIDPN                           NtStatus = 0xC01E0343
	STATUS_GRAPHICS_INVALID_PATH_IMPORTANCE_ORDINAL                    NtStatus = 0xC01E0344
	STATUS_GRAPHICS_INVALID_PATH_CONTENT_GEOMETRY_TRANSFORMATION       NtStatus = 0xC01E0345
	STATUS_GRAPHICS_PATH_CONTENT_GEOMETRY_TRANSFORMATION_NOT_SUPPORTED NtStatus = 0xC01E0346
	STATUS_GRAPHICS_INVALID_GAMMA_RAMP                                 NtStatus = 0xC01E0347
	STATUS_GRAPHICS_GAMMA_RAMP_NOT_SUPPORTED                           NtStatus = 0xC01E0348
	STATUS_GRAPHICS_MULTISAMPLING_NOT_SUPPORTED                        NtStatus = 0xC01E0349
	STATUS_GRAPHICS_MODE_NOT_IN_MODESET                                NtStatus = 0xC01E034A
	STATUS_GRAPHICS_INVALID_VIDPN_TOPOLOGY_RECOMMENDATION_REASON       NtStatus = 0xC01E034D
	STATUS_GRAPHICS_INVALID_PATH_CONTENT_TYPE                          NtStatus = 0xC01E034E
	STATUS_GRAPHICS_INVALID_COPYPROTECTION_TYPE                        NtStatus = 0xC01E034F
	STATUS_GRAPHICS_UNASSIGNED_MODESET_ALREADY_EXISTS                  NtStatus = 0xC01E0350
	STATUS_GRAPHICS_INVALID_SCANLINE_ORDERING                          NtStatus = 0xC01E0352
	STATUS_GRAPHICS_TOPOLOGY_CHANGES_NOT_ALLOWED                       NtStatus = 0xC01E0353
	STATUS_GRAPHICS_NO_AVAILABLE_IMPORTANCE_ORDINALS                   NtStatus = 0xC01E0354
	STATUS_GRAPHICS_INCOMPATIBLE_PRIVATE_FORMAT                        NtStatus = 0xC01E0355
	STATUS_GRAPHICS_INVALID_MODE_PRUNING_ALGORITHM                     NtStatus = 0xC01E0356
	STATUS_GRAPHICS_INVALID_MONITOR_CAPABILITY_ORIGIN                  NtStatus = 0xC01E0357
	STATUS_GRAPHICS_INVALID_MONITOR_FREQUENCYRANGE_CONSTRAINT          NtStatus = 0xC01E0358
	STATUS_GRAPHICS_MAX_NUM_PATHS_REACHED                              NtStatus = 0xC01E0359
	STATUS_GRAPHICS_CANCEL_VIDPN_TOPOLOGY_AUGMENTATION                 NtStatus = 0xC01E035A
	STATUS_GRAPHICS_INVALID_CLIENT_TYPE                                NtStatus = 0xC01E035B
	STATUS_GRAPHICS_CLIENTVIDPN_NOT_SET                                NtStatus = 0xC01E035C
	STATUS_GRAPHICS_SPECIFIED_CHILD_ALREADY_CONNECTED                  NtStatus = 0xC01E0400
	STATUS_GRAPHICS_CHILD_DESCRIPTOR_NOT_SUPPORTED                     NtStatus = 0xC01E0401
	STATUS_GRAPHICS_NOT_A_LINKED_ADAPTER                               NtStatus = 0xC01E0430
	STATUS_GRAPHICS_LEADLINK_NOT_ENUMERATED                            NtStatus = 0xC01E0431
	STATUS_GRAPHICS_CHAINLINKS_NOT_ENUMERATED                          NtStatus = 0xC01E0432
	STATUS_GRAPHICS_ADAPTER_CHAIN_NOT_READY                            NtStatus = 0xC01E0433
	STATUS_GRAPHICS_CHAINLINKS_NOT_STARTED                             NtStatus = 0xC01E0434
	STATUS_GRAPHICS_CHAINLINKS_NOT_POWERED_ON                          NtStatus = 0xC01E0435
	STATUS_GRAPHICS_INCONSISTENT_DEVICE_LINK_STATE                     NtStatus = 0xC01E0436
	STATUS_GRAPHICS_NOT_POST_DEVICE_DRIVER                             NtStatus = 0xC01E0438
	STATUS_GRAPHICS_ADAPTER_ACCESS_NOT_EXCLUDED                        NtStatus = 0xC01E043B
	STATUS_GRAPHICS_OPM_NOT_SUPPORTED                                  NtStatus = 0xC01E0500
	STATUS_GRAPHICS_COPP_NOT_SUPPORTED                                 NtStatus = 0xC01E0501
	STATUS_GRAPHICS_UAB_NOT_SUPPORTED                                  NtStatus = 0xC01E0502
	STATUS_GRAPHICS_OPM_INVALID_ENCRYPTED_PARAMETERS                   NtStatus = 0xC01E0503
	STATUS_GRAPHICS_OPM_PARAMETER_ARRAY_TOO_SMALL                      NtStatus = 0xC01E0504
	STATUS_GRAPHICS_OPM_NO_PROTECTED_OUTPUTS_EXIST                     NtStatus = 0xC01E0505
	STATUS_GRAPHICS_PVP_NO_DISPLAY_DEVICE_CORRESPONDS_TO_NAME          NtStatus = 0xC01E0506
	STATUS_GRAPHICS_PVP_DISPLAY_DEVICE_NOT_ATTACHED_TO_DESKTOP         NtStatus = 0xC01E0507
	STATUS_GRAPHICS_PVP_MIRRORING_DEVICES_NOT_SUPPORTED                NtStatus = 0xC01E0508
	STATUS_GRAPHICS_OPM_INVALID_POINTER                                NtStatus = 0xC01E050A
	STATUS_GRAPHICS_OPM_INTERNAL_ERROR                                 NtStatus = 0xC01E050B
	STATUS_GRAPHICS_OPM_INVALID_HANDLE                                 NtStatus = 0xC01E050C
	STATUS_GRAPHICS_PVP_NO_MONITORS_CORRESPOND_TO_DISPLAY_DEVICE       NtStatus = 0xC01E050D
	STATUS_GRAPHICS_PVP_INVALID_CERTIFICATE_LENGTH                     NtStatus = 0xC01E050E
	STATUS_GRAPHICS_OPM_SPANNING_MODE_ENABLED                          NtStatus = 0xC01E050F
	STATUS_GRAPHICS_OPM_THEATER_MODE_ENABLED                           NtStatus = 0xC01E0510
	STATUS_GRAPHICS_PVP_HFS_FAILED                                     NtStatus = 0xC01E0511
	STATUS_GRAPHICS_OPM_INVALID_SRM                                    NtStatus = 0xC01E0512
	STATUS_GRAPHICS_OPM_OUTPUT_DOES_NOT_SUPPORT_HDCP                   NtStatus = 0xC01E0513
	STATUS_GRAPHICS_OPM_OUTPUT_DOES_NOT_SUPPORT_ACP                    NtStatus = 0xC01E0514
	STATUS_GRAPHICS_OPM_OUTPUT_DOES_NOT_SUPPORT_CGMSA                  NtStatus = 0xC01E0515
	STATUS_GRAPHICS_OPM_HDCP_SRM_NEVER_SET                             NtStatus = 0xC01E0516
	STATUS_GRAPHICS_OPM_RESOLUTION_TOO_HIGH                            NtStatus = 0xC01E0517
	STATUS_GRAPHICS_OPM_ALL_HDCP_HARDWARE_ALREADY_IN_USE               NtStatus = 0xC01E0518
	STATUS_GRAPHICS_OPM_PROTECTED_OUTPUT_NO_LONGER_EXISTS              NtStatus = 0xC01E051A
	STATUS_GRAPHICS_OPM_SESSION_TYPE_CHANGE_IN_PROGRESS                NtStatus = 0xC01E051B
	STATUS_GRAPHICS_OPM_PROTECTED_OUTPUT_DOES_NOT_HAVE_COPP_SEMANTICS  NtStatus = 0xC01E051C
	STATUS_GRAPHICS_OPM_INVALID_INFORMATION_REQUEST                    NtStatus = 0xC01E051D
	STATUS_GRAPHICS_OPM_DRIVER_INTERNAL_ERROR                          NtStatus = 0xC01E051E
	STATUS_GRAPHICS_OPM_PROTECTED_OUTPUT_DOES_NOT_HAVE_OPM_SEMANTICS   NtStatus = 0xC01E051F
	STATUS_GRAPHICS_OPM_SIGNALING_NOT_SUPPORTED                        NtStatus = 0xC01E0520
	STATUS_GRAPHICS_OPM_INVALID_CONFIGURATION_REQUEST                  NtStatus = 0xC01E0521
	STATUS_GRAPHICS_I2C_NOT_SUPPORTED                                  NtStatus = 0xC01E0580
	STATUS_GRAPHICS_I2C_DEVICE_DOES_NOT_EXIST                          NtStatus = 0xC01E0581
	STATUS_GRAPHICS_I2C_ERROR_TRANSMITTING_DATA                        NtStatus = 0xC01E0582
	STATUS_GRAPHICS_I2C_ERROR_RECEIVING_DATA                           NtStatus = 0xC01E0583
	STATUS_GRAPHICS_DDCCI_VCP_NOT_SUPPORTED                            NtStatus = 0xC01E0584
	STATUS_GRAPHICS_DDCCI_INVALID_DATA                                 NtStatus = 0xC01E0585
	STATUS_GRAPHICS_DDCCI_MONITOR_RETURNED_INVALID_TIMING_STATUS_BYTE  NtStatus = 0xC01E0586
	STATUS_GRAPHICS_DDCCI_INVALID_CAPABILITIES_STRING                  NtStatus = 0xC01E0587
	STATUS_GRAPHICS_MCA_INTERNAL_ERROR                                 NtStatus = 0xC01E0588
	STATUS_GRAPHICS_DDCCI_INVALID_MESSAGE_COMMAND                      NtStatus = 0xC01E0589
	STATUS_GRAPHICS_DDCCI_INVALID_MESSAGE_LENGTH                       NtStatus = 0xC01E058A
	STATUS_GRAPHICS_DDCCI_INVALID_MESSAGE_CHECKSUM                     NtStatus = 0xC01E058B
	STATUS_GRAPHICS_INVALID_PHYSICAL_MONITOR_HANDLE                    NtStatus = 0xC01E058C
	STATUS_GRAPHICS_MONITOR_NO_LONGER_EXISTS                           NtStatus = 0xC01E058D
	STATUS_GRAPHICS_ONLY_CONSOLE_SESSION_SUPPORTED                     NtStatus = 0xC01E05E0
	STATUS_GRAPHICS_NO_DISPLAY_DEVICE_CORRESPONDS_TO_NAME              NtStatus = 0xC01E05E1
	STATUS_GRAPHICS_DISPLAY_DEVICE_NOT_ATTACHED_TO_DESKTOP             NtStatus = 0xC01E05E2
	STATUS_GRAPHICS_MIRRORING_DEVICES_NOT_SUPPORTED                    NtStatus = 0xC01E05E3
	STATUS_GRAPHICS_INVALID_POINTER                                    NtStatus = 0xC01E05E4
	STATUS_GRAPHICS_NO_MONITORS_CORRESPOND_TO_DISPLAY_DEVICE           NtStatus = 0xC01E05E5
	STATUS_GRAPHICS_PARAMETER_ARRAY_TOO_SMALL                          NtStatus = 0xC01E05E6
	STATUS_GRAPHICS_INTERNAL_ERROR                                     NtStatus = 0xC01E05E7
	STATUS_GRAPHICS_SESSION_TYPE_CHANGE_IN_PROGRESS                    NtStatus = 0xC01E05E8
	STATUS_FVE_LOCKED_VOLUME                                           NtStatus = 0xC0210000
	STATUS_FVE_NOT_ENCRYPTED                                           NtStatus = 0xC0210001
	STATUS_FVE_BAD_INFORMATION                                         NtStatus = 0xC0210002
	STATUS_FVE_TOO_SMALL                                               NtStatus = 0xC0210003
	STATUS_FVE_FAILED_WRONG_FS                                         NtStatus = 0xC0210004
	STATUS_FVE_FAILED_BAD_FS                                           NtStatus = 0xC0210005
	STATUS_FVE_FS_NOT_EXTENDED                                         NtStatus = 0xC0210006
	STATUS_FVE_FS_MOUNTED                                              NtStatus = 0xC0210007
	STATUS_FVE_NO_LICENSE                                              NtStatus = 0xC0210008
	STATUS_FVE_ACTION_NOT_ALLOWED                                      NtStatus = 0xC0210009
	STATUS_FVE_BAD_DATA                                                NtStatus = 0xC021000A
	STATUS_FVE_VOLUME_NOT_BOUND                                        NtStatus = 0xC021000B
	STATUS_FVE_NOT_DATA_VOLUME                                         NtStatus = 0xC021000C
	STATUS_FVE_CONV_READ_ERROR                                         NtStatus = 0xC021000D
	STATUS_FVE_CONV_WRITE_ERROR                                        NtStatus = 0xC021000E
	STATUS_FVE_OVERLAPPED_UPDATE                                       NtStatus = 0xC021000F
	STATUS_FVE_FAILED_SECTOR_SIZE                                      NtStatus = 0xC0210010
	STATUS_FVE_FAILED_AUTHENTICATION                                   NtStatus = 0xC0210011
	STATUS_FVE_NOT_OS_VOLUME                                           NtStatus = 0xC0210012
	STATUS_FVE_KEYFILE_NOT_FOUND                                       NtStatus = 0xC0210013
	STATUS_FVE_KEYFILE_INVALID                                         NtStatus = 0xC0210014
	STATUS_FVE_KEYFILE_NO_VMK                                          NtStatus = 0xC0210015
	STATUS_FVE_TPM_DISABLED                                            NtStatus = 0xC0210016
	STATUS_FVE_TPM_SRK_AUTH_NOT_ZERO                                   NtStatus = 0xC0210017
	STATUS_FVE_TPM_INVALID_PCR                                         NtStatus = 0xC0210018
	STATUS_FVE_TPM_NO_VMK                                              NtStatus = 0xC0210019
	STATUS_FVE_PIN_INVALID                                             NtStatus = 0xC021001A
	STATUS_FVE_AUTH_INVALID_APPLICATION                                NtStatus = 0xC021001B
	STATUS_FVE_AUTH_INVALID_CONFIG                                     NtStatus = 0xC021001C
	STATUS_FVE_DEBUGGER_ENABLED                                        NtStatus = 0xC021001D
	STATUS_FVE_DRY_RUN_FAILED                                          NtStatus = 0xC021001E
	STATUS_FVE_BAD_METADATA_POINTER                                    NtStatus = 0xC021001F
	STATUS_FVE_OLD_METADATA_COPY                                       NtStatus = 0xC0210020
	STATUS_FVE_REBOOT_REQUIRED                                         NtStatus = 0xC0210021
	STATUS_FVE_RAW_ACCESS                                              NtStatus = 0xC0210022
	STATUS_FVE_RAW_BLOCKED                                             NtStatus = 0xC0210023
	STATUS_FVE_NO_FEATURE_LICENSE                                      NtStatus = 0xC0210026
	STATUS_FVE_POLICY_USER_DISABLE_RDV_NOT_ALLOWED                     NtStatus = 0xC0210027
	STATUS_FVE_CONV_RECOVERY_FAILED                                    NtStatus = 0xC0210028
	STATUS_FVE_VIRTUALIZED_SPACE_TOO_BIG                               NtStatus = 0xC0210029
	STATUS_FVE_VOLUME_TOO_SMALL                                        NtStatus = 0xC0210030
	STATUS_FWP_CALLOUT_NOT_FOUND                                       NtStatus = 0xC0220001
	STATUS_FWP_CONDITION_NOT_FOUND                                     NtStatus = 0xC0220002
	STATUS_FWP_FILTER_NOT_FOUND                                        NtStatus = 0xC0220003
	STATUS_FWP_LAYER_NOT_FOUND                                         NtStatus = 0xC0220004
	STATUS_FWP_PROVIDER_NOT_FOUND                                      NtStatus = 0xC0220005
	STATUS_FWP_PROVIDER_CONTEXT_NOT_FOUND                              NtStatus = 0xC0220006
	STATUS_FWP_SUBLAYER_NOT_FOUND                                      NtStatus = 0xC0220007
	STATUS_FWP_NOT_FOUND                                               NtStatus = 0xC0220008
	STATUS_FWP_ALREADY_EXISTS                                          NtStatus = 0xC0220009
	STATUS_FWP_IN_USE                                                  NtStatus = 0xC022000A
	STATUS_FWP_DYNAMIC_SESSION_IN_PROGRESS                             NtStatus = 0xC022000B
	STATUS_FWP_WRONG_SESSION                                           NtStatus = 0xC022000C
	STATUS_FWP_NO_TXN_IN_PROGRESS                                      NtStatus = 0xC022000D
	STATUS_FWP_TXN_IN_PROGRESS                                         NtStatus = 0xC022000E
	STATUS_FWP_TXN_ABORTED                                             NtStatus = 0xC022000F
	STATUS_FWP_SESSION_ABORTED                                         NtStatus = 0xC0220010
	STATUS_FWP_INCOMPATIBLE_TXN                                        NtStatus = 0xC0220011
	STATUS_FWP_TIMEOUT                                                 NtStatus = 0xC0220012
	STATUS_FWP_NET_EVENTS_DISABLED                                     NtStatus = 0xC0220013
	STATUS_FWP_INCOMPATIBLE_LAYER                                      NtStatus = 0xC0220014
	STATUS_FWP_KM_CLIENTS_ONLY                                         NtStatus = 0xC0220015
	STATUS_FWP_LIFETIME_MISMATCH                                       NtStatus = 0xC0220016
	STATUS_FWP_BUILTIN_OBJECT                                          NtStatus = 0xC0220017
	STATUS_FWP_TOO_MANY_BOOTTIME_FILTERS                               NtStatus = 0xC0220018
	STATUS_FWP_TOO_MANY_CALLOUTS                                       NtStatus = 0xC0220018
	STATUS_FWP_NOTIFICATION_DROPPED                                    NtStatus = 0xC0220019
	STATUS_FWP_TRAFFIC_MISMATCH                                        NtStatus = 0xC022001A
	STATUS_FWP_INCOMPATIBLE_SA_STATE                                   NtStatus = 0xC022001B
	STATUS_FWP_NULL_POINTER                                            NtStatus = 0xC022001C
	STATUS_FWP_INVALID_ENUMERATOR                                      NtStatus = 0xC022001D
	STATUS_FWP_INVALID_FLAGS                                           NtStatus = 0xC022001E
	STATUS_FWP_INVALID_NET_MASK                                        NtStatus = 0xC022001F
	STATUS_FWP_INVALID_RANGE                                           NtStatus = 0xC0220020
	STATUS_FWP_INVALID_INTERVAL                                        NtStatus = 0xC0220021
	STATUS_FWP_ZERO_LENGTH_ARRAY                                       NtStatus = 0xC0220022
	STATUS_FWP_NULL_DISPLAY_NAME                                       NtStatus = 0xC0220023
	STATUS_FWP_INVALID_ACTION_TYPE                                     NtStatus = 0xC0220024
	STATUS_FWP_INVALID_WEIGHT                                          NtStatus = 0xC0220025
	STATUS_FWP_MATCH_TYPE_MISMATCH                                     NtStatus = 0xC0220026
	STATUS_FWP_TYPE_MISMATCH                                           NtStatus = 0xC0220027
	STATUS_FWP_OUT_OF_BOUNDS                                           NtStatus = 0xC0220028
	STATUS_FWP_RESERVED                                                NtStatus = 0xC0220029
	STATUS_FWP_DUPLICATE_CONDITION                                     NtStatus = 0xC022002A
	STATUS_FWP_DUPLICATE_KEYMOD                                        NtStatus = 0xC022002B
	STATUS_FWP_ACTION_INCOMPATIBLE_WITH_LAYER                          NtStatus = 0xC022002C
	STATUS_FWP_ACTION_INCOMPATIBLE_WITH_SUBLAYER                       NtStatus = 0xC022002D
	STATUS_FWP_CONTEXT_INCOMPATIBLE_WITH_LAYER                         NtStatus = 0xC022002E
	STATUS_FWP_CONTEXT_INCOMPATIBLE_WITH_CALLOUT                       NtStatus = 0xC022002F
	STATUS_FWP_INCOMPATIBLE_AUTH_METHOD                                NtStatus = 0xC0220030
	STATUS_FWP_INCOMPATIBLE_DH_GROUP                                   NtStatus = 0xC0220031
	STATUS_FWP_EM_NOT_SUPPORTED                                        NtStatus = 0xC0220032
	STATUS_FWP_NEVER_MATCH                                             NtStatus = 0xC0220033
	STATUS_FWP_PROVIDER_CONTEXT_MISMATCH                               NtStatus = 0xC0220034
	STATUS_FWP_INVALID_PARAMETER                                       NtStatus = 0xC0220035
	STATUS_FWP_TOO_MANY_SUBLAYERS                                      NtStatus = 0xC0220036
	STATUS_FWP_CALLOUT_NOTIFICATION_FAILED                             NtStatus = 0xC0220037
	STATUS_FWP_INCOMPATIBLE_AUTH_CONFIG                                NtStatus = 0xC0220038
	STATUS_FWP_INCOMPATIBLE_CIPHER_CONFIG                              NtStatus = 0xC0220039
	STATUS_FWP_DUPLICATE_AUTH_METHOD                                   NtStatus = 0xC022003C
	STATUS_FWP_TCPIP_NOT_READY                                         NtStatus = 0xC0220100
	STATUS_FWP_INJECT_HANDLE_CLOSING                                   NtStatus = 0xC0220101
	STATUS_FWP_INJECT_HANDLE_STALE                                     NtStatus = 0xC0220102
	STATUS_FWP_CANNOT_PEND                                             NtStatus = 0xC0220103
	STATUS_NDIS_CLOSING                                                NtStatus = 0xC0230002
	STATUS_NDIS_BAD_VERSION                                            NtStatus = 0xC0230004
	STATUS_NDIS_BAD_CHARACTERISTICS                                    NtStatus = 0xC0230005
	STATUS_NDIS_ADAPTER_NOT_FOUND                                      NtStatus = 0xC0230006
	STATUS_NDIS_OPEN_FAILED                                            NtStatus = 0xC0230007
	STATUS_NDIS_DEVICE_FAILED                                          NtStatus = 0xC0230008
	STATUS_NDIS_MULTICAST_FULL                                         NtStatus = 0xC0230009
	STATUS_NDIS_MULTICAST_EXISTS                                       NtStatus = 0xC023000A
	STATUS_NDIS_MULTICAST_NOT_FOUND                                    NtStatus = 0xC023000B
	STATUS_NDIS_REQUEST_ABORTED                                        NtStatus = 0xC023000C
	STATUS_NDIS_RESET_IN_PROGRESS                                      NtStatus = 0xC023000D
	STATUS_NDIS_INVALID_PACKET                                         NtStatus = 0xC023000F
	STATUS_NDIS_INVALID_DEVICE_REQUEST                                 NtStatus = 0xC0230010
	STATUS_NDIS_ADAPTER_NOT_READY                                      NtStatus = 0xC0230011
	STATUS_NDIS_INVALID_LENGTH                                         NtStatus = 0xC0230014
	STATUS_NDIS_INVALID_DATA                                           NtStatus = 0xC0230015
	STATUS_NDIS_BUFFER_TOO_SHORT                                       NtStatus = 0xC0230016
	STATUS_NDIS_INVALID_OID                                            NtStatus = 0xC0230017
	STATUS_NDIS_ADAPTER_REMOVED                                        NtStatus = 0xC0230018
	STATUS_NDIS_UNSUPPORTED_MEDIA                                      NtStatus = 0xC0230019
	STATUS_NDIS_GROUP_ADDRESS_IN_USE                                   NtStatus = 0xC023001A
	STATUS_NDIS_FILE_NOT_FOUND                                         NtStatus = 0xC023001B
	STATUS_NDIS_ERROR_READING_FILE                                     NtStatus = 0xC023001C
	STATUS_NDIS_ALREADY_MAPPED                                         NtStatus = 0xC023001D
	STATUS_NDIS_RESOURCE_CONFLICT                                      NtStatus = 0xC023001E
	STATUS_NDIS_MEDIA_DISCONNECTED                                     NtStatus = 0xC023001F
	STATUS_NDIS_INVALID_ADDRESS                                        NtStatus = 0xC0230022
	STATUS_NDIS_PAUSED                                                 NtStatus = 0xC023002A
	STATUS_NDIS_INTERFACE_NOT_FOUND                                    NtStatus = 0xC023002B
	STATUS_NDIS_UNSUPPORTED_REVISION                                   NtStatus = 0xC023002C
	STATUS_NDIS_INVALID_PORT                                           NtStatus = 0xC023002D
	STATUS_NDIS_INVALID_PORT_STATE                                     NtStatus = 0xC023002E
	STATUS_NDIS_LOW_POWER_STATE                                        NtStatus = 0xC023002F
	STATUS_NDIS_NOT_SUPPORTED                                          NtStatus = 0xC02300BB
	STATUS_NDIS_OFFLOAD_POLICY                                         NtStatus = 0xC023100F
	STATUS_NDIS_OFFLOAD_CONNECTION_REJECTED                            NtStatus = 0xC0231012
	STATUS_NDIS_OFFLOAD_PATH_REJECTED                                  NtStatus = 0xC0231013
	STATUS_NDIS_DOT11_AUTO_CONFIG_ENABLED                              NtStatus = 0xC0232000
	STATUS_NDIS_DOT11_MEDIA_IN_USE                                     NtStatus = 0xC0232001
	STATUS_NDIS_DOT11_POWER_STATE_INVALID                              NtStatus = 0xC0232002
	STATUS_NDIS_PM_WOL_PATTERN_LIST_FULL                               NtStatus = 0xC0232003
	STATUS_NDIS_PM_PROTOCOL_OFFLOAD_LIST_FULL                          NtStatus = 0xC0232004
	STATUS_IPSEC_BAD_SPI                                               NtStatus = 0xC0360001
	STATUS_IPSEC_SA_LIFETIME_EXPIRED                                   NtStatus = 0xC0360002
	STATUS_IPSEC_WRONG_SA                                              NtStatus = 0xC0360003
	STATUS_IPSEC_REPLAY_CHECK_FAILED                                   NtStatus = 0xC0360004
	STATUS_IPSEC_INVALID_PACKET                                        NtStatus = 0xC0360005
	STATUS_IPSEC_INTEGRITY_CHECK_FAILED                                NtStatus = 0xC0360006
	STATUS_IPSEC_CLEAR_TEXT_DROP                                       NtStatus = 0xC0360007
	STATUS_IPSEC_AUTH_FIREWALL_DROP                                    NtStatus = 0xC0360008
	STATUS_IPSEC_THROTTLE_DROP                                         NtStatus = 0xC0360009
	STATUS_IPSEC_DOSP_BLOCK                                            NtStatus = 0xC0368000
	STATUS_IPSEC_DOSP_RECEIVED_MULTICAST                               NtStatus = 0xC0368001
	STATUS_IPSEC_DOSP_INVALID_PACKET                                   NtStatus = 0xC0368002
	STATUS_IPSEC_DOSP_STATE_LOOKUP_FAILED                              NtStatus = 0xC0368003
	STATUS_IPSEC_DOSP_MAX_ENTRIES                                      NtStatus = 0xC0368004
	STATUS_IPSEC_DOSP_KEYMOD_NOT_ALLOWED                               NtStatus = 0xC0368005
	STATUS_IPSEC_DOSP_MAX_PER_IP_RATELIMIT_QUEUES                      NtStatus = 0xC0368006
	STATUS_VOLMGR_MIRROR_NOT_SUPPORTED                                 NtStatus = 0xC038005B
	STATUS_VOLMGR_RAID5_NOT_SUPPORTED                                  NtStatus = 0xC038005C
	STATUS_VIRTDISK_PROVIDER_NOT_FOUND                                 NtStatus = 0xC03A0014
	STATUS_VIRTDISK_NOT_VIRTUAL_DISK                                   NtStatus = 0xC03A0015
	STATUS_VHD_PARENT_VHD_ACCESS_DENIED                                NtStatus = 0xC03A0016
	STATUS_VHD_CHILD_PARENT_SIZE_MISMATCH                              NtStatus = 0xC03A0017
	STATUS_VHD_DIFFERENCING_CHAIN_CYCLE_DETECTED                       NtStatus = 0xC03A0018
	STATUS_VHD_DIFFERENCING_CHAIN_ERROR_IN_PARENT                      NtStatus = 0xC03A0019
)

var ntStatusStrings = map[NtStatus]string{
	STATUS_SUCCESS:                              "The operation completed successfully. ",
	STATUS_WAIT_1:                               "The caller specified WaitAny for WaitType and one of the dispatcher objects in the Object array has been set to the signaled state.",
	STATUS_WAIT_2:                               "The caller specified WaitAny for WaitType and one of the dispatcher objects in the Object array has been set to the signaled state.",
	STATUS_WAIT_3:                               "The caller specified WaitAny for WaitType and one of the dispatcher objects in the Object array has been set to the signaled state.",
	STATUS_WAIT_63:                              "The caller specified WaitAny for WaitType and one of the dispatcher objects in the Object array has been set to the signaled state.",
	STATUS_ABANDONED:                            "The caller attempted to wait for a mutex that has been abandoned.",
	STATUS_ABANDONED_WAIT_63:                    "The caller attempted to wait for a mutex that has been abandoned.",
	STATUS_USER_APC:                             "A user-mode APC was delivered before the given Interval expired.",
	STATUS_ALERTED:                              "The delay completed because the thread was alerted.",
	STATUS_TIMEOUT:                              "The given Timeout interval expired.",
	STATUS_PENDING:                              "The operation that was requested is pending completion.",
	STATUS_REPARSE:                              "A reparse should be performed by the Object Manager because the name of the file resulted in a symbolic link.",
	STATUS_MORE_ENTRIES:                         "Returned by enumeration APIs to indicate more information is available to successive calls.",
	STATUS_NOT_ALL_ASSIGNED:                     "Indicates not all privileges or groups that are referenced are assigned to the caller. This allows, for example, all privileges to be disabled without having to know exactly which privileges are assigned.",
	STATUS_SOME_NOT_MAPPED:                      "Some of the information to be translated has not been translated.",
	STATUS_OPLOCK_BREAK_IN_PROGRESS:             "An open/create operation completed while an opportunistic lock (oplock) break is underway.",
	STATUS_VOLUME_MOUNTED:                       "A new volume has been mounted by a file system.",
	STATUS_RXACT_COMMITTED:                      "This success level status indicates that the transaction state already exists for the registry subtree but that a transaction commit was previously aborted. The commit has now been completed.",
	STATUS_NOTIFY_CLEANUP:                       "Indicates that a notify change request has been completed due to closing the handle that made the notify change request.",
	STATUS_NOTIFY_ENUM_DIR:                      "Indicates that a notify change request is being completed and that the information is not being returned in the caller's buffer. The caller now needs to enumerate the files to find the changes.",
	STATUS_NO_QUOTAS_FOR_ACCOUNT:                "{No Quotas} No system quota limits are specifically set for this account.",
	STATUS_PRIMARY_TRANSPORT_CONNECT_FAILED:     "{Connect Failure on Primary Transport} An attempt was made to connect to the remote server %hs on the primary transport, but the connection failed. The computer WAS able to connect on a secondary transport.",
	STATUS_PAGE_FAULT_TRANSITION:                "The page fault was a transition fault.",
	STATUS_PAGE_FAULT_DEMAND_ZERO:               "The page fault was a demand zero fault.",
	STATUS_PAGE_FAULT_COPY_ON_WRITE:             "The page fault was a demand zero fault.",
	STATUS_PAGE_FAULT_GUARD_PAGE:                "The page fault was a demand zero fault.",
	STATUS_PAGE_FAULT_PAGING_FILE:               "The page fault was satisfied by reading from a secondary storage device.",
	STATUS_CACHE_PAGE_LOCKED:                    "The cached page was locked during operation.",
	STATUS_CRASH_DUMP:                           "The crash dump exists in a paging file.",
	STATUS_BUFFER_ALL_ZEROS:                     "The specified buffer contains all zeros.",
	STATUS_REPARSE_OBJECT:                       "A reparse should be performed by the Object Manager because the name of the file resulted in a symbolic link.",
	STATUS_RESOURCE_REQUIREMENTS_CHANGED:        "The device has succeeded a query-stop and its resource requirements have changed.",
	STATUS_TRANSLATION_COMPLETE:                 "The translator has translated these resources into the global space and no additional translations should be performed.",
	STATUS_DS_MEMBERSHIP_EVALUATED_LOCALLY:      "The directory service evaluated group memberships locally, because it was unable to contact a global catalog server.",
	STATUS_NOTHING_TO_TERMINATE:                 "A process being terminated has no threads to terminate.",
	STATUS_PROCESS_NOT_IN_JOB:                   "The specified process is not part of a job.",
	STATUS_PROCESS_IN_JOB:                       "The specified process is part of a job.",
	STATUS_VOLSNAP_HIBERNATE_READY:              "{Volume Shadow Copy Service} The system is now ready for hibernation.",
	STATUS_FSFILTER_OP_COMPLETED_SUCCESSFULLY:   "A file system or file system filter driver has successfully completed an FsFilter operation.",
	STATUS_INTERRUPT_VECTOR_ALREADY_CONNECTED:   "The specified interrupt vector was already connected.",
	STATUS_INTERRUPT_STILL_CONNECTED:            "The specified interrupt vector is still connected.",
	STATUS_PROCESS_CLONED:                       "The current process is a cloned process.",
	STATUS_FILE_LOCKED_WITH_ONLY_READERS:        "The file was locked and all users of the file can only read.",
	STATUS_FILE_LOCKED_WITH_WRITERS:             "The file was locked and at least one user of the file can write.",
	STATUS_RESOURCEMANAGER_READ_ONLY:            "The specified ResourceManager made no changes or updates to the resource under this transaction.",
	STATUS_WAIT_FOR_OPLOCK:                      "An operation is blocked and waiting for an oplock.",
	DBG_EXCEPTION_HANDLED:                       "Debugger handled the exception.",
	DBG_CONTINUE:                                "The debugger continued.",
	STATUS_FLT_IO_COMPLETE:                      "The IO was completed by a filter.",
	STATUS_FILE_NOT_AVAILABLE:                   "The file is temporarily unavailable.",
	STATUS_CALLBACK_RETURNED_THREAD_AFFINITY:    "A threadpool worker thread entered a callback at thread affinity %p and exited at affinity %p.This is unexpected, indicating that the callback missed restoring the priority.",
	STATUS_OBJECT_NAME_EXISTS:                   "{Object Exists} An attempt was made to create an object but the object name already exists.",
	STATUS_THREAD_WAS_SUSPENDED:                 "{Thread Suspended} A thread termination occurred while the thread was suspended. The thread resumed, and termination proceeded.",
	STATUS_WORKING_SET_LIMIT_RANGE:              "{Working Set Range Error} An attempt was made to set the working set minimum or maximum to values that are outside the allowable range.",
	STATUS_IMAGE_NOT_AT_BASE:                    "{Image Relocated} An image file could not be mapped at the address that is specified in the image file. Local fixes must be performed on this image.",
	STATUS_RXACT_STATE_CREATED:                  "This informational level status indicates that a specified registry subtree transaction state did not yet exist and had to be created.",
	STATUS_SEGMENT_NOTIFICATION:                 "{Segment Load} A virtual DOS machine (VDM) is loading, unloading, or moving an MS-DOS or Win16 program segment image. An exception is raised so that a debugger can load, unload, or track symbols and breakpoints within these 16-bit segments.",
	STATUS_LOCAL_USER_SESSION_KEY:               "{Local Session Key} A user session key was requested for a local remote procedure call (RPC) connection. The session key that is returned is a constant value and not unique to this connection.",
	STATUS_BAD_CURRENT_DIRECTORY:                "{Invalid Current Directory} The process cannot switch to the startup current directory %hs. Select OK to set the current directory to %hs, or select CANCEL to exit.",
	STATUS_SERIAL_MORE_WRITES:                   "{Serial IOCTL Complete} A serial I/O operation was completed by another write to a serial port. (The IOCTL_SERIAL_XOFF_COUNTER reached zero.)",
	STATUS_REGISTRY_RECOVERED:                   "{Registry Recovery} One of the files that contains the system registry data had to be recovered by using a log or alternate copy. The recovery was successful.",
	STATUS_FT_READ_RECOVERY_FROM_BACKUP:         "{Redundant Read} To satisfy a read request, the Windows NT operating system fault-tolerant file system successfully read the requested data from a redundant copy. This was done because the file system encountered a failure on a member of the fault-tolerant volume but was unable to reassign the failing area of the device.",
	STATUS_FT_WRITE_RECOVERY:                    "{Redundant Write} To satisfy a write request, the Windows NT fault-tolerant file system successfully wrote a redundant copy of the information. This was done because the file system encountered a failure on a member of the fault-tolerant volume but was unable to reassign the failing area of the device.",
	STATUS_SERIAL_COUNTER_TIMEOUT:               "{Serial IOCTL Timeout} A serial I/O operation completed because the time-out period expired. (The IOCTL_SERIAL_XOFF_COUNTER had not reached zero.)",
	STATUS_NULL_LM_PASSWORD:                     "{Password Too Complex} The Windows password is too complex to be converted to a LAN Manager password. The LAN Manager password that returned is a NULL string.",
	STATUS_IMAGE_MACHINE_TYPE_MISMATCH:          "{Machine Type Mismatch} The image file %hs is valid but is for a machine type other than the current machine. Select OK to continue, or CANCEL to fail the DLL load.",
	STATUS_RECEIVE_PARTIAL:                      "{Partial Data Received} The network transport returned partial data to its client. The remaining data will be sent later.",
	STATUS_RECEIVE_EXPEDITED:                    "{Expedited Data Received} The network transport returned data to its client that was marked as expedited by the remote system.",
	STATUS_RECEIVE_PARTIAL_EXPEDITED:            "{Partial Expedited Data Received} The network transport returned partial data to its client and this data was marked as expedited by the remote system. The remaining data will be sent later.",
	STATUS_EVENT_DONE:                           "{TDI Event Done} The TDI indication has completed successfully.",
	STATUS_EVENT_PENDING:                        "{TDI Event Pending} The TDI indication has entered the pending state.",
	STATUS_CHECKING_FILE_SYSTEM:                 "Checking file system on %wZ.",
	STATUS_FATAL_APP_EXIT:                       "{Fatal Application Exit} %hs",
	STATUS_PREDEFINED_HANDLE:                    "The specified registry key is referenced by a predefined handle.",
	STATUS_WAS_UNLOCKED:                         "{Page Unlocked} The page protection of a locked page was changed to 'No Access' and the page was unlocked from memory and from the process.",
	STATUS_SERVICE_NOTIFICATION:                 "%hs",
	STATUS_WAS_LOCKED:                           "{Page Locked} One of the pages to lock was already locked.",
	STATUS_LOG_HARD_ERROR:                       "Application popup: %1 : %2",
	STATUS_ALREADY_WIN32:                        "A Win32 process already exists.",
	STATUS_WX86_UNSIMULATE:                      "An exception status code that is used by the Win32 x86 emulation subsystem.",
	STATUS_WX86_CONTINUE:                        "An exception status code that is used by the Win32 x86 emulation subsystem.",
	STATUS_WX86_SINGLE_STEP:                     "An exception status code that is used by the Win32 x86 emulation subsystem.",
	STATUS_WX86_BREAKPOINT:                      "An exception status code that is used by the Win32 x86 emulation subsystem.",
	STATUS_WX86_EXCEPTION_CONTINUE:              "An exception status code that is used by the Win32 x86 emulation subsystem.",
	STATUS_WX86_EXCEPTION_LASTCHANCE:            "An exception status code that is used by the Win32 x86 emulation subsystem.",
	STATUS_WX86_EXCEPTION_CHAIN:                 "An exception status code that is used by the Win32 x86 emulation subsystem.",
	STATUS_IMAGE_MACHINE_TYPE_MISMATCH_EXE:      "{Machine Type Mismatch} The image file %hs is valid but is for a machine type other than the current machine.",
	STATUS_NO_YIELD_PERFORMED:                   "A yield execution was performed and no thread was available to run.",
	STATUS_TIMER_RESUME_IGNORED:                 "The resume flag to a timer API was ignored.",
	STATUS_ARBITRATION_UNHANDLED:                "The arbiter has deferred arbitration of these resources to its parent.",
	STATUS_CARDBUS_NOT_SUPPORTED:                "The device has detected a CardBus card in its slot.",
	STATUS_WX86_CREATEWX86TIB:                   "An exception status code that is used by the Win32 x86 emulation subsystem.",
	STATUS_MP_PROCESSOR_MISMATCH:                "The CPUs in this multiprocessor system are not all the same revision level. To use all processors, the operating system restricts itself to the features of the least capable processor in the system. If problems occur with this system, contact the CPU manufacturer to see if this mix of processors is supported.",
	STATUS_HIBERNATED:                           "The system was put into hibernation.",
	STATUS_RESUME_HIBERNATION:                   "The system was resumed from hibernation.",
	STATUS_FIRMWARE_UPDATED:                     "Windows has detected that the system firmware (BIOS) was updated [previous firmware date = %2, current firmware date %3].",
	STATUS_DRIVERS_LEAKING_LOCKED_PAGES:         "A device driver is leaking locked I/O pages and is causing system degradation. The system has automatically enabled the tracking code to try and catch the culprit.",
	STATUS_MESSAGE_RETRIEVED:                    "The ALPC message being canceled has already been retrieved from the queue on the other side.",
	STATUS_SYSTEM_POWERSTATE_TRANSITION:         "The system power state is transitioning from %2 to %3.",
	STATUS_ALPC_CHECK_COMPLETION_LIST:           "The receive operation was successful. Check the ALPC completion list for the received message.",
	STATUS_SYSTEM_POWERSTATE_COMPLEX_TRANSITION: "The system power state is transitioning from %2 to %3 but could enter %4.",
	STATUS_ACCESS_AUDIT_BY_POLICY:               "Access to %1 is monitored by policy rule %2.",
	STATUS_ABANDON_HIBERFILE:                    "A valid hibernation file has been invalidated and should be abandoned.",
	STATUS_BIZRULES_NOT_ENABLED:                 "Business rule scripts are disabled for the calling application.",
	STATUS_WAKE_SYSTEM:                          "The system has awoken.",
	STATUS_DS_SHUTTING_DOWN:                     "The directory service is shutting down.",
	DBG_REPLY_LATER:                             "Debugger will reply later.",
	DBG_UNABLE_TO_PROVIDE_HANDLE:                "Debugger cannot provide a handle.",
	DBG_TERMINATE_THREAD:                        "Debugger terminated the thread.",
	DBG_TERMINATE_PROCESS:                       "Debugger terminated the process.",
	DBG_CONTROL_C:                               "Debugger obtained control of C.",
	DBG_PRINTEXCEPTION_C:                        "Debugger printed an exception on control C.",
	DBG_RIPEXCEPTION:                            "Debugger received a RIP exception.",
	DBG_CONTROL_BREAK:                           "Debugger received a control break.",
	DBG_COMMAND_EXCEPTION:                       "Debugger command communication exception.",
	RPC_NT_UUID_LOCAL_ONLY:                      "A UUID that is valid only on this computer has been allocated.",
	RPC_NT_SEND_INCOMPLETE:                      "Some data remains to be sent in the request buffer.",
	STATUS_CTX_CDM_CONNECT:                      "The Client Drive Mapping Service has connected on Terminal Connection.",
	STATUS_CTX_CDM_DISCONNECT:                   "The Client Drive Mapping Service has disconnected on Terminal Connection.",
	STATUS_SXS_RELEASE_ACTIVATION_CONTEXT:       "A kernel mode component is releasing a reference on an activation context.",
	STATUS_RECOVERY_NOT_NEEDED:                  "The transactional resource manager is already consistent. Recovery is not needed.",
	STATUS_RM_ALREADY_STARTED:                   "The transactional resource manager has already been started.",
	STATUS_LOG_NO_RESTART:                       "The log service encountered a log stream with no restart area.",
	STATUS_VIDEO_DRIVER_DEBUG_REPORT_REQUEST:    "{Display Driver Recovered From Failure} The %hs display driver has detected a failure and recovered from it. Some graphical operations may have failed. The next time you restart the machine, a dialog box appears, giving you an opportunity to upload data about this failure to Microsoft.",
	STATUS_GRAPHICS_PARTIAL_DATA_POPULATED:      "The specified buffer is not big enough to contain the entire requested dataset. Partial data is populated up to the size of the buffer.The caller needs to provide a buffer of the size as specified in the partially populated buffer's content (interface specific).",
	STATUS_GRAPHICS_DRIVER_MISMATCH:             "The kernel driver detected a version mismatch between it and the user mode driver.",
	STATUS_GRAPHICS_MODE_NOT_PINNED:             "No mode is pinned on the specified VidPN source/target.",
	STATUS_GRAPHICS_NO_PREFERRED_MODE:           "The specified mode set does not specify a preference for one of its modes.",
	STATUS_GRAPHICS_DATASET_IS_EMPTY:            "The specified dataset (for example, mode set, frequency range set, descriptor set, or topology) is empty.",
	STATUS_GRAPHICS_NO_MORE_ELEMENTS_IN_DATASET: "The specified dataset (for example, mode set, frequency range set, descriptor set, or topology) does not contain any more elements.",
	STATUS_GRAPHICS_PATH_CONTENT_GEOMETRY_TRANSFORMATION_NOT_PINNED:    "The specified content transformation is not pinned on the specified VidPN present path.",
	STATUS_GRAPHICS_UNKNOWN_CHILD_STATUS:                               "The child device presence was not reliably detected.",
	STATUS_GRAPHICS_LEADLINK_START_DEFERRED:                            "Starting the lead adapter in a linked configuration has been temporarily deferred.",
	STATUS_GRAPHICS_POLLING_TOO_FREQUENTLY:                             "The display adapter is being polled for children too frequently at the same polling level.",
	STATUS_GRAPHICS_START_DEFERRED:                                     "Starting the adapter has been temporarily deferred.",
	STATUS_NDIS_INDICATION_REQUIRED:                                    "The request will be completed later by an NDIS status indication.",
	STATUS_GUARD_PAGE_VIOLATION:                                        "{EXCEPTION} Guard Page Exception A page of memory that marks the end of a data structure, such as a stack or an array, has been accessed.",
	STATUS_DATATYPE_MISALIGNMENT:                                       "{EXCEPTION} Alignment Fault A data type misalignment was detected in a load or store instruction.",
	STATUS_BREAKPOINT:                                                  "{EXCEPTION} Breakpoint A breakpoint has been reached.",
	STATUS_SINGLE_STEP:                                                 "{EXCEPTION} Single Step A single step or trace operation has just been completed.",
	STATUS_BUFFER_OVERFLOW:                                             "{Buffer Overflow} The data was too large to fit into the specified buffer.",
	STATUS_NO_MORE_FILES:                                               "{No More Files} No more files were found which match the file specification.",
	STATUS_WAKE_SYSTEM_DEBUGGER:                                        "{Kernel Debugger Awakened} The system debugger was awakened by an interrupt.",
	STATUS_HANDLES_CLOSED:                                              "{Handles Closed} Handles to objects have been automatically closed because of the requested operation.",
	STATUS_NO_INHERITANCE:                                              "{Non-Inheritable ACL} An access control list (ACL) contains no components that can be inherited.",
	STATUS_GUID_SUBSTITUTION_MADE:                                      "{GUID Substitution} During the translation of a globally unique identifier (GUID) to a Windows security ID (SID), no administratively defined GUID prefix was found. A substitute prefix was used, which will not compromise system security. However, this may provide a more restrictive access than intended.",
	STATUS_PARTIAL_COPY:                                                "Because of protection conflicts, not all the requested bytes could be copied.",
	STATUS_DEVICE_PAPER_EMPTY:                                          "{Out of Paper} The printer is out of paper.",
	STATUS_DEVICE_POWERED_OFF:                                          "{Device Power Is Off} The printer power has been turned off.",
	STATUS_DEVICE_OFF_LINE:                                             "{Device Offline} The printer has been taken offline.",
	STATUS_DEVICE_BUSY:                                                 "{Device Busy} The device is currently busy.",
	STATUS_NO_MORE_EAS:                                                 "{No More EAs} No more extended attributes (EAs) were found for the file.",
	STATUS_INVALID_EA_NAME:                                             "{Illegal EA} The specified extended attribute (EA) name contains at least one illegal character.",
	STATUS_EA_LIST_INCONSISTENT:                                        "{Inconsistent EA List} The extended attribute (EA) list is inconsistent.",
	STATUS_INVALID_EA_FLAG:                                             "{Invalid EA Flag} An invalid extended attribute (EA) flag was set.",
	STATUS_VERIFY_REQUIRED:                                             "{Verifying Disk} The media has changed and a verify operation is in progress; therefore, no reads or writes may be performed to the device, except those that are used in the verify operation.",
	STATUS_EXTRANEOUS_INFORMATION:                                      "{Too Much Information} The specified access control list (ACL) contained more information than was expected.",
	STATUS_RXACT_COMMIT_NECESSARY:                                      "This warning level status indicates that the transaction state already exists for the registry subtree, but that a transaction commit was previously aborted. The commit has NOT been completed but has not been rolled back either; therefore, it may still be committed, if needed.",
	STATUS_NO_MORE_ENTRIES:                                             "{No More Entries} No more entries are available from an enumeration operation.",
	STATUS_FILEMARK_DETECTED:                                           "{Filemark Found} A filemark was detected.",
	STATUS_MEDIA_CHANGED:                                               "{Media Changed} The media may have changed.",
	STATUS_BUS_RESET:                                                   "{I/O Bus Reset} An I/O bus reset was detected.",
	STATUS_END_OF_MEDIA:                                                "{End of Media} The end of the media was encountered.",
	STATUS_BEGINNING_OF_MEDIA:                                          "The beginning of a tape or partition has been detected.",
	STATUS_MEDIA_CHECK:                                                 "{Media Changed} The media may have changed.",
	STATUS_SETMARK_DETECTED:                                            "A tape access reached a set mark.",
	STATUS_NO_DATA_DETECTED:                                            "During a tape access, the end of the data written is reached.",
	STATUS_REDIRECTOR_HAS_OPEN_HANDLES:                                 "The redirector is in use and cannot be unloaded.",
	STATUS_SERVER_HAS_OPEN_HANDLES:                                     "The server is in use and cannot be unloaded.",
	STATUS_ALREADY_DISCONNECTED:                                        "The specified connection has already been disconnected.",
	STATUS_LONGJUMP:                                                    "A long jump has been executed.",
	STATUS_CLEANER_CARTRIDGE_INSTALLED:                                 "A cleaner cartridge is present in the tape library.",
	STATUS_PLUGPLAY_QUERY_VETOED:                                       "The Plug and Play query operation was not successful.",
	STATUS_UNWIND_CONSOLIDATE:                                          "A frame consolidation has been executed.",
	STATUS_REGISTRY_HIVE_RECOVERED:                                     "{Registry Hive Recovered} The registry hive (file): %hs was corrupted and it has been recovered. Some data might have been lost.",
	STATUS_DLL_MIGHT_BE_INSECURE:                                       "The application is attempting to run executable code from the module %hs. This may be insecure. An alternative, %hs, is available. Should the application use the secure module %hs?",
	STATUS_DLL_MIGHT_BE_INCOMPATIBLE:                                   "The application is loading executable code from the module %hs. This is secure but may be incompatible with previous releases of the operating system. An alternative, %hs, is available. Should the application use the secure module %hs?",
	STATUS_STOPPED_ON_SYMLINK:                                          "The create operation stopped after reaching a symbolic link.",
	STATUS_DEVICE_REQUIRES_CLEANING:                                    "The device has indicated that cleaning is necessary.",
	STATUS_DEVICE_DOOR_OPEN:                                            "The device has indicated that its door is open. Further operations require it closed and secured.",
	STATUS_DATA_LOST_REPAIR:                                            "Windows discovered a corruption in the file %hs. This file has now been repaired. Check if any data in the file was lost because of the corruption.",
	DBG_EXCEPTION_NOT_HANDLED:                                          "Debugger did not handle the exception.",
	STATUS_CLUSTER_NODE_ALREADY_UP:                                     "The cluster node is already up.",
	STATUS_CLUSTER_NODE_ALREADY_DOWN:                                   "The cluster node is already down.",
	STATUS_CLUSTER_NETWORK_ALREADY_ONLINE:                              "The cluster network is already online.",
	STATUS_CLUSTER_NETWORK_ALREADY_OFFLINE:                             "The cluster network is already offline.",
	STATUS_CLUSTER_NODE_ALREADY_MEMBER:                                 "The cluster node is already a member of the cluster.",
	STATUS_COULD_NOT_RESIZE_LOG:                                        "The log could not be set to the requested size.",
	STATUS_NO_TXF_METADATA:                                             "There is no transaction metadata on the file.",
	STATUS_CANT_RECOVER_WITH_HANDLE_OPEN:                               "The file cannot be recovered because there is a handle still open on it.",
	STATUS_TXF_METADATA_ALREADY_PRESENT:                                "Transaction metadata is already present on this file and cannot be superseded.",
	STATUS_TRANSACTION_SCOPE_CALLBACKS_NOT_SET:                         "A transaction scope could not be entered because the scope handler has not been initialized.",
	STATUS_VIDEO_HUNG_DISPLAY_DRIVER_THREAD_RECOVERED:                  "{Display Driver Stopped Responding and recovered} The %hs display driver has stopped working normally. The recovery had been performed.",
	STATUS_FLT_BUFFER_TOO_SMALL:                                        "{Buffer too small} The buffer is too small to contain the entry. No information has been written to the buffer.",
	STATUS_FVE_PARTIAL_METADATA:                                        "Volume metadata read or write is incomplete.",
	STATUS_FVE_TRANSIENT_STATE:                                         "BitLocker encryption keys were ignored because the volume was in a transient state.",
	STATUS_UNSUCCESSFUL:                                                "{Operation Failed} The requested operation was unsuccessful.",
	STATUS_NOT_IMPLEMENTED:                                             "{Not Implemented} The requested operation is not implemented.",
	STATUS_INVALID_INFO_CLASS:                                          "{Invalid Parameter} The specified information class is not a valid information class for the specified object.",
	STATUS_INFO_LENGTH_MISMATCH:                                        "The specified information record length does not match the length that is required for the specified information class.",
	STATUS_ACCESS_VIOLATION:                                            "The instruction at 0x%08lx referenced memory at 0x%08lx. The memory could not be %s.",
	STATUS_IN_PAGE_ERROR:                                               "The instruction at 0x%08lx referenced memory at 0x%08lx. The required data was not placed into memory because of an I/O error status of 0x%08lx.",
	STATUS_PAGEFILE_QUOTA:                                              "The page file quota for the process has been exhausted.",
	STATUS_INVALID_HANDLE:                                              "An invalid HANDLE was specified.",
	STATUS_BAD_INITIAL_STACK:                                           "An invalid initial stack was specified in a call to NtCreateThread.",
	STATUS_BAD_INITIAL_PC:                                              "An invalid initial start address was specified in a call to NtCreateThread.",
	STATUS_INVALID_CID:                                                 "An invalid client ID was specified.",
	STATUS_TIMER_NOT_CANCELED:                                          "An attempt was made to cancel or set a timer that has an associated APC and the specified thread is not the thread that originally set the timer with an associated APC routine.",
	STATUS_INVALID_PARAMETER:                                           "An invalid parameter was passed to a service or function.",
	STATUS_NO_SUCH_DEVICE:                                              "A device that does not exist was specified.",
	STATUS_NO_SUCH_FILE:                                                "{File Not Found} The file %hs does not exist.",
	STATUS_INVALID_DEVICE_REQUEST:                                      "The specified request is not a valid operation for the target device.",
	STATUS_END_OF_FILE:                                                 "The end-of-file marker has been reached. There is no valid data in the file beyond this marker.",
	STATUS_WRONG_VOLUME:                                                "{Wrong Volume} The wrong volume is in the drive. Insert volume %hs into drive %hs.",
	STATUS_NO_MEDIA_IN_DEVICE:                                          "{No Disk} There is no disk in the drive. Insert a disk into drive %hs.",
	STATUS_UNRECOGNIZED_MEDIA:                                          "{Unknown Disk Format} The disk in drive %hs is not formatted properly. Check the disk, and reformat it, if needed.",
	STATUS_NONEXISTENT_SECTOR:                                          "{Sector Not Found} The specified sector does not exist.",
	STATUS_MORE_PROCESSING_REQUIRED:                                    "{Still Busy} The specified I/O request packet (IRP) cannot be disposed of because the I/O operation is not complete.",
	STATUS_NO_MEMORY:                                                   "{Not Enough Quota} Not enough virtual memory or paging file quota is available to complete the specified operation.",
	STATUS_CONFLICTING_ADDRESSES:                                       "{Conflicting Address Range} The specified address range conflicts with the address space.",
	STATUS_NOT_MAPPED_VIEW:                                             "The address range to unmap is not a mapped view.",
	STATUS_UNABLE_TO_FREE_VM:                                           "The virtual memory cannot be freed.",
	STATUS_UNABLE_TO_DELETE_SECTION:                                    "The specified section cannot be deleted.",
	STATUS_INVALID_SYSTEM_SERVICE:                                      "An invalid system service was specified in a system service call.",
	STATUS_ILLEGAL_INSTRUCTION:                                         "{EXCEPTION} Illegal Instruction An attempt was made to execute an illegal instruction.",
	STATUS_INVALID_LOCK_SEQUENCE:                                       "{Invalid Lock Sequence} An attempt was made to execute an invalid lock sequence.",
	STATUS_INVALID_VIEW_SIZE:                                           "{Invalid Mapping} An attempt was made to create a view for a section that is bigger than the section.",
	STATUS_INVALID_FILE_FOR_SECTION:                                    "{Bad File} The attributes of the specified mapping file for a section of memory cannot be read.",
	STATUS_ALREADY_COMMITTED:                                           "{Already Committed} The specified address range is already committed.",
	STATUS_ACCESS_DENIED:                                               "{Access Denied} A process has requested access to an object but has not been granted those access rights.",
	STATUS_BUFFER_TOO_SMALL:                                            "{Buffer Too Small} The buffer is too small to contain the entry. No information has been written to the buffer.",
	STATUS_OBJECT_TYPE_MISMATCH:                                        "{Wrong Type} There is a mismatch between the type of object that is required by the requested operation and the type of object that is specified in the request.",
	STATUS_NONCONTINUABLE_EXCEPTION:                                    "{EXCEPTION} Cannot Continue Windows cannot continue from this exception.",
	STATUS_INVALID_DISPOSITION:                                         "An invalid exception disposition was returned by an exception handler.",
	STATUS_UNWIND:                                                      "Unwind exception code.",
	STATUS_BAD_STACK:                                                   "An invalid or unaligned stack was encountered during an unwind operation.",
	STATUS_INVALID_UNWIND_TARGET:                                       "An invalid unwind target was encountered during an unwind operation.",
	STATUS_NOT_LOCKED:                                                  "An attempt was made to unlock a page of memory that was not locked.",
	STATUS_PARITY_ERROR:                                                "A device parity error on an I/O operation.",
	STATUS_UNABLE_TO_DECOMMIT_VM:                                       "An attempt was made to decommit uncommitted virtual memory.",
	STATUS_NOT_COMMITTED:                                               "An attempt was made to change the attributes on memory that has not been committed.",
	STATUS_INVALID_PORT_ATTRIBUTES:                                     "Invalid object attributes specified to NtCreatePort or invalid port attributes specified to NtConnectPort.",
	STATUS_PORT_MESSAGE_TOO_LONG:                                       "The length of the message that was passed to NtRequestPort or NtRequestWaitReplyPort is longer than the maximum message that is allowed by the port.",
	STATUS_INVALID_PARAMETER_MIX:                                       "An invalid combination of parameters was specified.",
	STATUS_INVALID_QUOTA_LOWER:                                         "An attempt was made to lower a quota limit below the current usage.",
	STATUS_DISK_CORRUPT_ERROR:                                          "{Corrupt Disk} The file system structure on the disk is corrupt and unusable. Run the Chkdsk utility on the volume %hs.",
	STATUS_OBJECT_NAME_INVALID:                                         "The object name is invalid.",
	STATUS_OBJECT_NAME_NOT_FOUND:                                       "The object name is not found.",
	STATUS_OBJECT_NAME_COLLISION:                                       "The object name already exists.",
	STATUS_PORT_DISCONNECTED:                                           "An attempt was made to send a message to a disconnected communication port.",
	STATUS_DEVICE_ALREADY_ATTACHED:                                     "An attempt was made to attach to a device that was already attached to another device.",
	STATUS_OBJECT_PATH_INVALID:                                         "The object path component was not a directory object.",
	STATUS_OBJECT_PATH_NOT_FOUND:                                       "{Path Not Found} The path %hs does not exist.",
	STATUS_OBJECT_PATH_SYNTAX_BAD:                                      "The object path component was not a directory object.",
	STATUS_DATA_OVERRUN:                                                "{Data Overrun} A data overrun error occurred.",
	STATUS_DATA_LATE_ERROR:                                             "{Data Late} A data late error occurred.",
	STATUS_DATA_ERROR:                                                  "{Data Error} An error occurred in reading or writing data.",
	STATUS_CRC_ERROR:                                                   "{Bad CRC} A cyclic redundancy check (CRC) checksum error occurred.",
	STATUS_SECTION_TOO_BIG:                                             "{Section Too Large} The specified section is too big to map the file.",
	STATUS_PORT_CONNECTION_REFUSED:                                     "The NtConnectPort request is refused.",
	STATUS_INVALID_PORT_HANDLE:                                         "The type of port handle is invalid for the operation that is requested.",
	STATUS_SHARING_VIOLATION:                                           "A file cannot be opened because the share access flags are incompatible.",
	STATUS_QUOTA_EXCEEDED:                                              "Insufficient quota exists to complete the operation.",
	STATUS_INVALID_PAGE_PROTECTION:                                     "The specified page protection was not valid.",
	STATUS_MUTANT_NOT_OWNED:                                            "An attempt to release a mutant object was made by a thread that was not the owner of the mutant object.",
	STATUS_SEMAPHORE_LIMIT_EXCEEDED:                                    "An attempt was made to release a semaphore such that its maximum count would have been exceeded.",
	STATUS_PORT_ALREADY_SET:                                            "An attempt was made to set the DebugPort or ExceptionPort of a process, but a port already exists in the process, or an attempt was made to set the CompletionPort of a file but a port was already set in the file, or an attempt was made to set the associated completion port of an ALPC port but it is already set.",
	STATUS_SECTION_NOT_IMAGE:                                           "An attempt was made to query image information on a section that does not map an image.",
	STATUS_SUSPEND_COUNT_EXCEEDED:                                      "An attempt was made to suspend a thread whose suspend count was at its maximum.",
	STATUS_THREAD_IS_TERMINATING:                                       "An attempt was made to suspend a thread that has begun termination.",
	STATUS_BAD_WORKING_SET_LIMIT:                                       "An attempt was made to set the working set limit to an invalid value (for example, the minimum greater than maximum).",
	STATUS_INCOMPATIBLE_FILE_MAP:                                       "A section was created to map a file that is not compatible with an already existing section that maps the same file.",
	STATUS_SECTION_PROTECTION:                                          "A view to a section specifies a protection that is incompatible with the protection of the initial view.",
	STATUS_EAS_NOT_SUPPORTED:                                           "An operation involving EAs failed because the file system does not support EAs.",
	STATUS_EA_TOO_LARGE:                                                "An EA operation failed because the EA set is too large.",
	STATUS_NONEXISTENT_EA_ENTRY:                                        "An EA operation failed because the name or EA index is invalid.",
	STATUS_NO_EAS_ON_FILE:                                              "The file for which EAs were requested has no EAs.",
	STATUS_EA_CORRUPT_ERROR:                                            "The EA is corrupt and cannot be read.",
	STATUS_FILE_LOCK_CONFLICT:                                          "A requested read/write cannot be granted due to a conflicting file lock.",
	STATUS_LOCK_NOT_GRANTED:                                            "A requested file lock cannot be granted due to other existing locks.",
	STATUS_DELETE_PENDING:                                              "A non-close operation has been requested of a file object that has a delete pending.",
	STATUS_CTL_FILE_NOT_SUPPORTED:                                      "An attempt was made to set the control attribute on a file. This attribute is not supported in the destination file system.",
	STATUS_UNKNOWN_REVISION:                                            "Indicates a revision number that was encountered or specified is not one that is known by the service. It may be a more recent revision than the service is aware of.",
	STATUS_REVISION_MISMATCH:                                           "Indicates that two revision levels are incompatible.",
	STATUS_INVALID_OWNER:                                               "Indicates a particular security ID may not be assigned as the owner of an object.",
	STATUS_INVALID_PRIMARY_GROUP:                                       "Indicates a particular security ID may not be assigned as the primary group of an object.",
	STATUS_NO_IMPERSONATION_TOKEN:                                      "An attempt has been made to operate on an impersonation token by a thread that is not currently impersonating a client.",
	STATUS_CANT_DISABLE_MANDATORY:                                      "A mandatory group may not be disabled.",
	STATUS_NO_LOGON_SERVERS:                                            "No logon servers are currently available to service the logon request.",
	STATUS_NO_SUCH_LOGON_SESSION:                                       "A specified logon session does not exist. It may already have been terminated.",
	STATUS_NO_SUCH_PRIVILEGE:                                           "A specified privilege does not exist.",
	STATUS_PRIVILEGE_NOT_HELD:                                          "A required privilege is not held by the client.",
	STATUS_INVALID_ACCOUNT_NAME:                                        "The name provided is not a properly formed account name.",
	STATUS_USER_EXISTS:                                                 "The specified account already exists.",
	STATUS_NO_SUCH_USER:                                                "The specified account does not exist.",
	STATUS_GROUP_EXISTS:                                                "The specified group already exists.",
	STATUS_NO_SUCH_GROUP:                                               "The specified group does not exist.",
	STATUS_MEMBER_IN_GROUP:                                             "The specified user account is already in the specified group account. Also used to indicate a group cannot be deleted because it contains a member.",
	STATUS_MEMBER_NOT_IN_GROUP:                                         "The specified user account is not a member of the specified group account.",
	STATUS_LAST_ADMIN:                                                  "Indicates the requested operation would disable or delete the last remaining administration account. This is not allowed to prevent creating a situation in which the system cannot be administrated.",
	STATUS_WRONG_PASSWORD:                                              "When trying to update a password, this return status indicates that the value provided as the current password is not correct.",
	STATUS_ILL_FORMED_PASSWORD:                                         "When trying to update a password, this return status indicates that the value provided for the new password contains values that are not allowed in passwords.",
	STATUS_PASSWORD_RESTRICTION:                                        "When trying to update a password, this status indicates that some password update rule has been violated. For example, the password may not meet length criteria.",
	STATUS_LOGON_FAILURE:                                               "The attempted logon is invalid. This is either due to a bad username or authentication information.",
	STATUS_ACCOUNT_RESTRICTION:                                         "Indicates a referenced user name and authentication information are valid, but some user account restriction has prevented successful authentication (such as time-of-day restrictions).",
	STATUS_INVALID_LOGON_HOURS:                                         "The user account has time restrictions and may not be logged onto at this time.",
	STATUS_INVALID_WORKSTATION:                                         "The user account is restricted so that it may not be used to log on from the source workstation.",
	STATUS_PASSWORD_EXPIRED:                                            "The user account password has expired.",
	STATUS_ACCOUNT_DISABLED:                                            "The referenced account is currently disabled and may not be logged on to.",
	STATUS_NONE_MAPPED:                                                 "None of the information to be translated has been translated.",
	STATUS_TOO_MANY_LUIDS_REQUESTED:                                    "The number of LUIDs requested may not be allocated with a single allocation.",
	STATUS_LUIDS_EXHAUSTED:                                             "Indicates there are no more LUIDs to allocate.",
	STATUS_INVALID_SUB_AUTHORITY:                                       "Indicates the sub-authority value is invalid for the particular use.",
	STATUS_INVALID_ACL:                                                 "Indicates the ACL structure is not valid.",
	STATUS_INVALID_SID:                                                 "Indicates the SID structure is not valid.",
	STATUS_INVALID_SECURITY_DESCR:                                      "Indicates the SECURITY_DESCRIPTOR structure is not valid.",
	STATUS_PROCEDURE_NOT_FOUND:                                         "Indicates the specified procedure address cannot be found in the DLL.",
	STATUS_INVALID_IMAGE_FORMAT:                                        "{Bad Image} %hs is either not designed to run on Windows or it contains an error. Try installing the program again using the original installation media or contact your system administrator or the software vendor for support.",
	STATUS_NO_TOKEN:                                                    "An attempt was made to reference a token that does not exist. This is typically done by referencing the token that is associated with a thread when the thread is not impersonating a client.",
	STATUS_BAD_INHERITANCE_ACL:                                         "Indicates that an attempt to build either an inherited ACL or ACE was not successful. This can be caused by a number of things. One of the more probable causes is the replacement of a CreatorId with a SID that did not fit into the ACE or ACL.",
	STATUS_RANGE_NOT_LOCKED:                                            "The range specified in NtUnlockFile was not locked.",
	STATUS_DISK_FULL:                                                   "An operation failed because the disk was full.",
	STATUS_SERVER_DISABLED:                                             "The GUID allocation server is disabled at the moment.",
	STATUS_SERVER_NOT_DISABLED:                                         "The GUID allocation server is enabled at the moment.",
	STATUS_TOO_MANY_GUIDS_REQUESTED:                                    "Too many GUIDs were requested from the allocation server at once.",
	STATUS_GUIDS_EXHAUSTED:                                             "The GUIDs could not be allocated because the Authority Agent was exhausted.",
	STATUS_INVALID_ID_AUTHORITY:                                        "The value provided was an invalid value for an identifier authority.",
	STATUS_AGENTS_EXHAUSTED:                                            "No more authority agent values are available for the particular identifier authority value.",
	STATUS_INVALID_VOLUME_LABEL:                                        "An invalid volume label has been specified.",
	STATUS_SECTION_NOT_EXTENDED:                                        "A mapped section could not be extended.",
	STATUS_NOT_MAPPED_DATA:                                             "Specified section to flush does not map a data file.",
	STATUS_RESOURCE_DATA_NOT_FOUND:                                     "Indicates the specified image file did not contain a resource section.",
	STATUS_RESOURCE_TYPE_NOT_FOUND:                                     "Indicates the specified resource type cannot be found in the image file.",
	STATUS_RESOURCE_NAME_NOT_FOUND:                                     "Indicates the specified resource name cannot be found in the image file.",
	STATUS_ARRAY_BOUNDS_EXCEEDED:                                       "{EXCEPTION} Array bounds exceeded.",
	STATUS_FLOAT_DENORMAL_OPERAND:                                      "{EXCEPTION} Floating-point denormal operand.",
	STATUS_FLOAT_DIVIDE_BY_ZERO:                                        "{EXCEPTION} Floating-point division by zero.",
	STATUS_FLOAT_INEXACT_RESULT:                                        "{EXCEPTION} Floating-point inexact result.",
	STATUS_FLOAT_INVALID_OPERATION:                                     "{EXCEPTION} Floating-point invalid operation.",
	STATUS_FLOAT_OVERFLOW:                                              "{EXCEPTION} Floating-point overflow.",
	STATUS_FLOAT_STACK_CHECK:                                           "{EXCEPTION} Floating-point stack check.",
	STATUS_FLOAT_UNDERFLOW:                                             "{EXCEPTION} Floating-point underflow.",
	STATUS_INTEGER_DIVIDE_BY_ZERO:                                      "{EXCEPTION} Integer division by zero.",
	STATUS_INTEGER_OVERFLOW:                                            "{EXCEPTION} Integer overflow.",
	STATUS_PRIVILEGED_INSTRUCTION:                                      "{EXCEPTION} Privileged instruction.",
	STATUS_TOO_MANY_PAGING_FILES:                                       "An attempt was made to install more paging files than the system supports.",
	STATUS_FILE_INVALID:                                                "The volume for a file has been externally altered such that the opened file is no longer valid.",
	STATUS_ALLOTTED_SPACE_EXCEEDED:                                     "When a block of memory is allotted for future updates, such as the memory allocated to hold discretionary access control and primary group information, successive updates may exceed the amount of memory originally allotted. Because a quota may already have been charged to several processes that have handles to the object, it is not reasonable to alter the size of the allocated memory. Instead, a request that requires more memory than has been allotted must fail and the STATUS_ALLOTTED_SPACE_EXCEEDED error returned.",
	STATUS_INSUFFICIENT_RESOURCES:                                      "Insufficient system resources exist to complete the API.",
	STATUS_DFS_EXIT_PATH_FOUND:                                         "An attempt has been made to open a DFS exit path control file.",
	STATUS_DEVICE_DATA_ERROR:                                           "There are bad blocks (sectors) on the hard disk.",
	STATUS_DEVICE_NOT_CONNECTED:                                        "There is bad cabling, non-termination, or the controller is not able to obtain access to the hard disk.",
	STATUS_FREE_VM_NOT_AT_BASE:                                         "Virtual memory cannot be freed because the base address is not the base of the region and a region size of zero was specified.",
	STATUS_MEMORY_NOT_ALLOCATED:                                        "An attempt was made to free virtual memory that is not allocated.",
	STATUS_WORKING_SET_QUOTA:                                           "The working set is not big enough to allow the requested pages to be locked.",
	STATUS_MEDIA_WRITE_PROTECTED:                                       "{Write Protect Error} The disk cannot be written to because it is write-protected. Remove the write protection from the volume %hs in drive %hs.",
	STATUS_DEVICE_NOT_READY:                                            "{Drive Not Ready} The drive is not ready for use; its door may be open. Check drive %hs and make sure that a disk is inserted and that the drive door is closed.",
	STATUS_INVALID_GROUP_ATTRIBUTES:                                    "The specified attributes are invalid or are incompatible with the attributes for the group as a whole.",
	STATUS_BAD_IMPERSONATION_LEVEL:                                     "A specified impersonation level is invalid. Also used to indicate that a required impersonation level was not provided.",
	STATUS_CANT_OPEN_ANONYMOUS:                                         "An attempt was made to open an anonymous-level token. Anonymous tokens may not be opened.",
	STATUS_BAD_VALIDATION_CLASS:                                        "The validation information class requested was invalid.",
	STATUS_BAD_TOKEN_TYPE:                                              "The type of a token object is inappropriate for its attempted use.",
	STATUS_BAD_MASTER_BOOT_RECORD:                                      "The type of a token object is inappropriate for its attempted use.",
	STATUS_INSTRUCTION_MISALIGNMENT:                                    "An attempt was made to execute an instruction at an unaligned address and the host system does not support unaligned instruction references.",
	STATUS_INSTANCE_NOT_AVAILABLE:                                      "The maximum named pipe instance count has been reached.",
	STATUS_PIPE_NOT_AVAILABLE:                                          "An instance of a named pipe cannot be found in the listening state.",
	STATUS_INVALID_PIPE_STATE:                                          "The named pipe is not in the connected or closing state.",
	STATUS_PIPE_BUSY:                                                   "The specified pipe is set to complete operations and there are current I/O operations queued so that it cannot be changed to queue operations.",
	STATUS_ILLEGAL_FUNCTION:                                            "The specified handle is not open to the server end of the named pipe.",
	STATUS_PIPE_DISCONNECTED:                                           "The specified named pipe is in the disconnected state.",
	STATUS_PIPE_CLOSING:                                                "The specified named pipe is in the closing state.",
	STATUS_PIPE_CONNECTED:                                              "The specified named pipe is in the connected state.",
	STATUS_PIPE_LISTENING:                                              "The specified named pipe is in the listening state.",
	STATUS_INVALID_READ_MODE:                                           "The specified named pipe is not in message mode.",
	STATUS_IO_TIMEOUT:                                                  "{Device Timeout} The specified I/O operation on %hs was not completed before the time-out period expired.",
	STATUS_FILE_FORCED_CLOSED:                                          "The specified file has been closed by another process.",
	STATUS_PROFILING_NOT_STARTED:                                       "Profiling is not started.",
	STATUS_PROFILING_NOT_STOPPED:                                       "Profiling is not stopped.",
	STATUS_COULD_NOT_INTERPRET:                                         "The passed ACL did not contain the minimum required information.",
	STATUS_FILE_IS_A_DIRECTORY:                                         "The file that was specified as a target is a directory, and the caller specified that it could be anything but a directory.",
	STATUS_NOT_SUPPORTED:                                               "The request is not supported.",
	STATUS_REMOTE_NOT_LISTENING:                                        "This remote computer is not listening.",
	STATUS_DUPLICATE_NAME:                                              "A duplicate name exists on the network.",
	STATUS_BAD_NETWORK_PATH:                                            "The network path cannot be located.",
	STATUS_NETWORK_BUSY:                                                "The network is busy.",
	STATUS_DEVICE_DOES_NOT_EXIST:                                       "This device does not exist.",
	STATUS_TOO_MANY_COMMANDS:                                           "The network BIOS command limit has been reached.",
	STATUS_ADAPTER_HARDWARE_ERROR:                                      "An I/O adapter hardware error has occurred.",
	STATUS_INVALID_NETWORK_RESPONSE:                                    "The network responded incorrectly.",
	STATUS_UNEXPECTED_NETWORK_ERROR:                                    "An unexpected network error occurred.",
	STATUS_BAD_REMOTE_ADAPTER:                                          "The remote adapter is not compatible.",
	STATUS_PRINT_QUEUE_FULL:                                            "The print queue is full.",
	STATUS_NO_SPOOL_SPACE:                                              "Space to store the file that is waiting to be printed is not available on the server.",
	STATUS_PRINT_CANCELLED:                                             "The requested print file has been canceled.",
	STATUS_NETWORK_NAME_DELETED:                                        "The network name was deleted.",
	STATUS_NETWORK_ACCESS_DENIED:                                       "Network access is denied.",
	STATUS_BAD_DEVICE_TYPE:                                             "{Incorrect Network Resource Type} The specified device type (LPT, for example) conflicts with the actual device type on the remote resource.",
	STATUS_BAD_NETWORK_NAME:                                            "{Network Name Not Found} The specified share name cannot be found on the remote server.",
	STATUS_TOO_MANY_NAMES:                                              "The name limit for the network adapter card of the local computer was exceeded.",
	STATUS_TOO_MANY_SESSIONS:                                           "The network BIOS session limit was exceeded.",
	STATUS_SHARING_PAUSED:                                              "File sharing has been temporarily paused.",
	STATUS_REQUEST_NOT_ACCEPTED:                                        "No more connections can be made to this remote computer at this time because the computer has already accepted the maximum number of connections.",
	STATUS_REDIRECTOR_PAUSED:                                           "Print or disk redirection is temporarily paused.",
	STATUS_NET_WRITE_FAULT:                                             "A network data fault occurred.",
	STATUS_PROFILING_AT_LIMIT:                                          "The number of active profiling objects is at the maximum and no more may be started.",
	STATUS_NOT_SAME_DEVICE:                                             "{Incorrect Volume} The destination file of a rename request is located on a different device than the source of the rename request.",
	STATUS_FILE_RENAMED:                                                "The specified file has been renamed and thus cannot be modified.",
	STATUS_VIRTUAL_CIRCUIT_CLOSED:                                      "{Network Request Timeout} The session with a remote server has been disconnected because the time-out interval for a request has expired.",
	STATUS_NO_SECURITY_ON_OBJECT:                                       "Indicates an attempt was made to operate on the security of an object that does not have security associated with it.",
	STATUS_CANT_WAIT:                                                   "Used to indicate that an operation cannot continue without blocking for I/O.",
	STATUS_PIPE_EMPTY:                                                  "Used to indicate that a read operation was done on an empty pipe.",
	STATUS_CANT_ACCESS_DOMAIN_INFO:                                     "Configuration information could not be read from the domain controller, either because the machine is unavailable or access has been denied.",
	STATUS_CANT_TERMINATE_SELF:                                         "Indicates that a thread attempted to terminate itself by default (called NtTerminateThread with NULL) and it was the last thread in the current process.",
	STATUS_INVALID_SERVER_STATE:                                        "Indicates the Sam Server was in the wrong state to perform the desired operation.",
	STATUS_INVALID_DOMAIN_STATE:                                        "Indicates the domain was in the wrong state to perform the desired operation.",
	STATUS_INVALID_DOMAIN_ROLE:                                         "This operation is only allowed for the primary domain controller of the domain.",
	STATUS_NO_SUCH_DOMAIN:                                              "The specified domain did not exist.",
	STATUS_DOMAIN_EXISTS:                                               "The specified domain already exists.",
	STATUS_DOMAIN_LIMIT_EXCEEDED:                                       "An attempt was made to exceed the limit on the number of domains per server for this release.",
	STATUS_OPLOCK_NOT_GRANTED:                                          "An error status returned when the opportunistic lock (oplock) request is denied.",
	STATUS_INVALID_OPLOCK_PROTOCOL:                                     "An error status returned when an invalid opportunistic lock (oplock) acknowledgment is received by a file system.",
	STATUS_INTERNAL_DB_CORRUPTION:                                      "This error indicates that the requested operation cannot be completed due to a catastrophic media failure or an on-disk data structure corruption.",
	STATUS_INTERNAL_ERROR:                                              "An internal error occurred.",
	STATUS_GENERIC_NOT_MAPPED:                                          "Indicates generic access types were contained in an access mask which should already be mapped to non-generic access types.",
	STATUS_BAD_DESCRIPTOR_FORMAT:                                       "Indicates a security descriptor is not in the necessary format (absolute or self-relative).",
	STATUS_INVALID_USER_BUFFER:                                         "An access to a user buffer failed at an expected point in time. This code is defined because the caller does not want to accept STATUS_ACCESS_VIOLATION in its filter.",
	STATUS_UNEXPECTED_IO_ERROR:                                         "If an I/O error that is not defined in the standard FsRtl filter is returned, it is converted to the following error, which is guaranteed to be in the filter. In this case, information is lost; however, the filter correctly handles the exception.",
	STATUS_UNEXPECTED_MM_CREATE_ERR:                                    "If an MM error that is not defined in the standard FsRtl filter is returned, it is converted to one of the following errors, which are guaranteed to be in the filter. In this case, information is lost; however, the filter correctly handles the exception.",
	STATUS_UNEXPECTED_MM_MAP_ERROR:                                     "If an MM error that is not defined in the standard FsRtl filter is returned, it is converted to one of the following errors, which are guaranteed to be in the filter. In this case, information is lost; however, the filter correctly handles the exception.",
	STATUS_UNEXPECTED_MM_EXTEND_ERR:                                    "If an MM error that is not defined in the standard FsRtl filter is returned, it is converted to one of the following errors, which are guaranteed to be in the filter. In this case, information is lost; however, the filter correctly handles the exception.",
	STATUS_NOT_LOGON_PROCESS:                                           "The requested action is restricted for use by logon processes only. The calling process has not registered as a logon process.",
	STATUS_LOGON_SESSION_EXISTS:                                        "An attempt has been made to start a new session manager or LSA logon session by using an ID that is already in use.",
	STATUS_INVALID_PARAMETER_1:                                         "An invalid parameter was passed to a service or function as the first argument.",
	STATUS_INVALID_PARAMETER_2:                                         "An invalid parameter was passed to a service or function as the second argument.",
	STATUS_INVALID_PARAMETER_3:                                         "An invalid parameter was passed to a service or function as the third argument.",
	STATUS_INVALID_PARAMETER_4:                                         "An invalid parameter was passed to a service or function as the fourth argument.",
	STATUS_INVALID_PARAMETER_5:                                         "An invalid parameter was passed to a service or function as the fifth argument.",
	STATUS_INVALID_PARAMETER_6:                                         "An invalid parameter was passed to a service or function as the sixth argument.",
	STATUS_INVALID_PARAMETER_7:                                         "An invalid parameter was passed to a service or function as the seventh argument.",
	STATUS_INVALID_PARAMETER_8:                                         "An invalid parameter was passed to a service or function as the eighth argument.",
	STATUS_INVALID_PARAMETER_9:                                         "An invalid parameter was passed to a service or function as the ninth argument.",
	STATUS_INVALID_PARAMETER_10:                                        "An invalid parameter was passed to a service or function as the tenth argument.",
	STATUS_INVALID_PARAMETER_11:                                        "An invalid parameter was passed to a service or function as the eleventh argument.",
	STATUS_INVALID_PARAMETER_12:                                        "An invalid parameter was passed to a service or function as the twelfth argument.",
	STATUS_REDIRECTOR_NOT_STARTED:                                      "An attempt was made to access a network file, but the network software was not yet started.",
	STATUS_REDIRECTOR_STARTED:                                          "An attempt was made to start the redirector, but the redirector has already been started.",
	STATUS_STACK_OVERFLOW:                                              "A new guard page for the stack cannot be created.",
	STATUS_NO_SUCH_PACKAGE:                                             "A specified authentication package is unknown.",
	STATUS_BAD_FUNCTION_TABLE:                                          "A malformed function table was encountered during an unwind operation.",
	STATUS_VARIABLE_NOT_FOUND:                                          "Indicates the specified environment variable name was not found in the specified environment block.",
	STATUS_DIRECTORY_NOT_EMPTY:                                         "Indicates that the directory trying to be deleted is not empty.",
	STATUS_FILE_CORRUPT_ERROR:                                          "{Corrupt File} The file or directory %hs is corrupt and unreadable. Run the Chkdsk utility.",
	STATUS_NOT_A_DIRECTORY:                                             "A requested opened file is not a directory.",
	STATUS_BAD_LOGON_SESSION_STATE:                                     "The logon session is not in a state that is consistent with the requested operation.",
	STATUS_LOGON_SESSION_COLLISION:                                     "An internal LSA error has occurred. An authentication package has requested the creation of a logon session but the ID of an already existing logon session has been specified.",
	STATUS_NAME_TOO_LONG:                                               "A specified name string is too long for its intended use.",
	STATUS_FILES_OPEN:                                                  "The user attempted to force close the files on a redirected drive, but there were opened files on the drive, and the user did not specify a sufficient level of force.",
	STATUS_CONNECTION_IN_USE:                                           "The user attempted to force close the files on a redirected drive, but there were opened directories on the drive, and the user did not specify a sufficient level of force.",
	STATUS_MESSAGE_NOT_FOUND:                                           "RtlFindMessage could not locate the requested message ID in the message table resource.",
	STATUS_PROCESS_IS_TERMINATING:                                      "An attempt was made to duplicate an object handle into or out of an exiting process.",
	STATUS_INVALID_LOGON_TYPE:                                          "Indicates an invalid value has been provided for the LogonType requested.",
	STATUS_NO_GUID_TRANSLATION:                                         "Indicates that an attempt was made to assign protection to a file system file or directory and one of the SIDs in the security descriptor could not be translated into a GUID that could be stored by the file system. This causes the protection attempt to fail, which may cause a file creation attempt to fail.",
	STATUS_CANNOT_IMPERSONATE:                                          "Indicates that an attempt has been made to impersonate via a named pipe that has not yet been read from.",
	STATUS_IMAGE_ALREADY_LOADED:                                        "Indicates that the specified image is already loaded.",
	STATUS_NO_LDT:                                                      "Indicates that an attempt was made to change the size of the LDT for a process that has no LDT.",
	STATUS_INVALID_LDT_SIZE:                                            "Indicates that an attempt was made to grow an LDT by setting its size, or that the size was not an even number of selectors.",
	STATUS_INVALID_LDT_OFFSET:                                          "Indicates that the starting value for the LDT information was not an integral multiple of the selector size.",
	STATUS_INVALID_LDT_DESCRIPTOR:                                      "Indicates that the user supplied an invalid descriptor when trying to set up LDT descriptors.",
	STATUS_INVALID_IMAGE_NE_FORMAT:                                     "The specified image file did not have the correct format. It appears to be NE format.",
	STATUS_RXACT_INVALID_STATE:                                         "Indicates that the transaction state of a registry subtree is incompatible with the requested operation. For example, a request has been made to start a new transaction with one already in progress, or a request has been made to apply a transaction when one is not currently in progress.",
	STATUS_RXACT_COMMIT_FAILURE:                                        "Indicates an error has occurred during a registry transaction commit. The database has been left in an unknown, but probably inconsistent, state. The state of the registry transaction is left as COMMITTING.",
	STATUS_MAPPED_FILE_SIZE_ZERO:                                       "An attempt was made to map a file of size zero with the maximum size specified as zero.",
	STATUS_TOO_MANY_OPENED_FILES:                                       "Too many files are opened on a remote server. This error should only be returned by the Windows redirector on a remote drive.",
	STATUS_CANCELLED:                                                   "The I/O request was canceled.",
	STATUS_CANNOT_DELETE:                                               "An attempt has been made to remove a file or directory that cannot be deleted.",
	STATUS_INVALID_COMPUTER_NAME:                                       "Indicates a name that was specified as a remote computer name is syntactically invalid.",
	STATUS_FILE_DELETED:                                                "An I/O request other than close was performed on a file after it was deleted, which can only happen to a request that did not complete before the last handle was closed via NtClose.",
	STATUS_SPECIAL_ACCOUNT:                                             "Indicates an operation that is incompatible with built-in accounts has been attempted on a built-in (special) SAM account. For example, built-in accounts cannot be deleted.",
	STATUS_SPECIAL_GROUP:                                               "The operation requested may not be performed on the specified group because it is a built-in special group.",
	STATUS_SPECIAL_USER:                                                "The operation requested may not be performed on the specified user because it is a built-in special user.",
	STATUS_MEMBERS_PRIMARY_GROUP:                                       "Indicates a member cannot be removed from a group because the group is currently the member's primary group.",
	STATUS_FILE_CLOSED:                                                 "An I/O request other than close and several other special case operations was attempted using a file object that had already been closed.",
	STATUS_TOO_MANY_THREADS:                                            "Indicates a process has too many threads to perform the requested action. For example, assignment of a primary token may only be performed when a process has zero or one threads.",
	STATUS_THREAD_NOT_IN_PROCESS:                                       "An attempt was made to operate on a thread within a specific process, but the specified thread is not in the specified process.",
	STATUS_TOKEN_ALREADY_IN_USE:                                        "An attempt was made to establish a token for use as a primary token but the token is already in use. A token can only be the primary token of one process at a time.",
	STATUS_PAGEFILE_QUOTA_EXCEEDED:                                     "The page file quota was exceeded.",
	STATUS_COMMITMENT_LIMIT:                                            "{Out of Virtual Memory} Your system is low on virtual memory. To ensure that Windows runs correctly, increase the size of your virtual memory paging file. For more information, see Help.",
	STATUS_INVALID_IMAGE_LE_FORMAT:                                     "The specified image file did not have the correct format: it appears to be LE format.",
	STATUS_INVALID_IMAGE_NOT_MZ:                                        "The specified image file did not have the correct format: it did not have an initial MZ.",
	STATUS_INVALID_IMAGE_PROTECT:                                       "The specified image file did not have the correct format: it did not have a proper e_lfarlc in the MZ header.",
	STATUS_INVALID_IMAGE_WIN_16:                                        "The specified image file did not have the correct format: it appears to be a 16-bit Windows image.",
	STATUS_LOGON_SERVER_CONFLICT:                                       "The Netlogon service cannot start because another Netlogon service running in the domain conflicts with the specified role.",
	STATUS_TIME_DIFFERENCE_AT_DC:                                       "The time at the primary domain controller is different from the time at the backup domain controller or member server by too large an amount.",
	STATUS_SYNCHRONIZATION_REQUIRED:                                    "The SAM database on a Windows Server operating system is significantly out of synchronization with the copy on the domain controller. A complete synchronization is required.",
	STATUS_DLL_NOT_FOUND:                                               "{Unable To Locate Component} This application has failed to start because %hs was not found. Reinstalling the application may fix this problem.",
	STATUS_OPEN_FAILED:                                                 "The NtCreateFile API failed. This error should never be returned to an application; it is a place holder for the Windows LAN Manager Redirector to use in its internal error-mapping routines.",
	STATUS_IO_PRIVILEGE_FAILED:                                         "{Privilege Failed} The I/O permissions for the process could not be changed.",
	STATUS_ORDINAL_NOT_FOUND:                                           "{Ordinal Not Found} The ordinal %ld could not be located in the dynamic link library %hs.",
	STATUS_ENTRYPOINT_NOT_FOUND:                                        "{Entry Point Not Found} The procedure entry point %hs could not be located in the dynamic link library %hs.",
	STATUS_CONTROL_C_EXIT:                                              "{Application Exit by CTRL+C} The application terminated as a result of a CTRL+C.",
	STATUS_LOCAL_DISCONNECT:                                            "{Virtual Circuit Closed} The network transport on your computer has closed a network connection. There may or may not be I/O requests outstanding.",
	STATUS_REMOTE_DISCONNECT:                                           "{Virtual Circuit Closed} The network transport on a remote computer has closed a network connection. There may or may not be I/O requests outstanding.",
	STATUS_REMOTE_RESOURCES:                                            "{Insufficient Resources on Remote Computer} The remote computer has insufficient resources to complete the network request. For example, the remote computer may not have enough available memory to carry out the request at this time.",
	STATUS_LINK_FAILED:                                                 "{Virtual Circuit Closed} An existing connection (virtual circuit) has been broken at the remote computer. There is probably something wrong with the network software protocol or the network hardware on the remote computer.",
	STATUS_LINK_TIMEOUT:                                                "{Virtual Circuit Closed} The network transport on your computer has closed a network connection because it had to wait too long for a response from the remote computer.",
	STATUS_INVALID_CONNECTION:                                          "The connection handle that was given to the transport was invalid.",
	STATUS_INVALID_ADDRESS:                                             "The address handle that was given to the transport was invalid.",
	STATUS_DLL_INIT_FAILED:                                             "{DLL Initialization Failed} Initialization of the dynamic link library %hs failed. The process is terminating abnormally.",
	STATUS_MISSING_SYSTEMFILE:                                          "{Missing System File} The required system file %hs is bad or missing.",
	STATUS_UNHANDLED_EXCEPTION:                                         "{Application Error} The exception %s (0x%08lx) occurred in the application at location 0x%08lx.",
	STATUS_APP_INIT_FAILURE:                                            "{Application Error} The application failed to initialize properly (0x%lx). Click OK to terminate the application.",
	STATUS_PAGEFILE_CREATE_FAILED:                                      "{Unable to Create Paging File} The creation of the paging file %hs failed (%lx). The requested size was %ld.",
	STATUS_NO_PAGEFILE:                                                 "{No Paging File Specified} No paging file was specified in the system configuration.",
	STATUS_INVALID_LEVEL:                                               "{Incorrect System Call Level} An invalid level was passed into the specified system call.",
	STATUS_WRONG_PASSWORD_CORE:                                         "{Incorrect Password to LAN Manager Server} You specified an incorrect password to a LAN Manager 2.x or MS-NET server.",
	STATUS_ILLEGAL_FLOAT_CONTEXT:                                       "{EXCEPTION} A real-mode application issued a floating-point instruction and floating-point hardware is not present.",
	STATUS_PIPE_BROKEN:                                                 "The pipe operation has failed because the other end of the pipe has been closed.",
	STATUS_REGISTRY_CORRUPT:                                            "{The Registry Is Corrupt} The structure of one of the files that contains registry data is corrupt; the image of the file in memory is corrupt; or the file could not be recovered because the alternate copy or log was absent or corrupt.",
	STATUS_REGISTRY_IO_FAILED:                                          "An I/O operation initiated by the Registry failed and cannot be recovered. The registry could not read in, write out, or flush one of the files that contain the system's image of the registry.",
	STATUS_NO_EVENT_PAIR:                                               "An event pair synchronization operation was performed using the thread-specific client/server event pair object, but no event pair object was associated with the thread.",
	STATUS_UNRECOGNIZED_VOLUME:                                         "The volume does not contain a recognized file system. Be sure that all required file system drivers are loaded and that the volume is not corrupt.",
	STATUS_SERIAL_NO_DEVICE_INITED:                                     "No serial device was successfully initialized. The serial driver will unload.",
	STATUS_NO_SUCH_ALIAS:                                               "The specified local group does not exist.",
	STATUS_MEMBER_NOT_IN_ALIAS:                                         "The specified account name is not a member of the group.",
	STATUS_MEMBER_IN_ALIAS:                                             "The specified account name is already a member of the group.",
	STATUS_ALIAS_EXISTS:                                                "The specified local group already exists.",
	STATUS_LOGON_NOT_GRANTED:                                           "A requested type of logon (for example, interactive, network, and service) is not granted by the local security policy of the target system. Ask the system administrator to grant the necessary form of logon.",
	STATUS_TOO_MANY_SECRETS:                                            "The maximum number of secrets that may be stored in a single system was exceeded. The length and number of secrets is limited to satisfy U.S. State Department export restrictions.",
	STATUS_SECRET_TOO_LONG:                                             "The length of a secret exceeds the maximum allowable length. The length and number of secrets is limited to satisfy U.S. State Department export restrictions.",
	STATUS_INTERNAL_DB_ERROR:                                           "The local security authority (LSA) database contains an internal inconsistency.",
	STATUS_FULLSCREEN_MODE:                                             "The requested operation cannot be performed in full-screen mode.",
	STATUS_TOO_MANY_CONTEXT_IDS:                                        "During a logon attempt, the user's security context accumulated too many security IDs. This is a very unusual situation. Remove the user from some global or local groups to reduce the number of security IDs to incorporate into the security context.",
	STATUS_LOGON_TYPE_NOT_GRANTED:                                      "A user has requested a type of logon (for example, interactive or network) that has not been granted. An administrator has control over who may logon interactively and through the network.",
	STATUS_NOT_REGISTRY_FILE:                                           "The system has attempted to load or restore a file into the registry, and the specified file is not in the format of a registry file.",
	STATUS_NT_CROSS_ENCRYPTION_REQUIRED:                                "An attempt was made to change a user password in the security account manager without providing the necessary Windows cross-encrypted password.",
	STATUS_DOMAIN_CTRLR_CONFIG_ERROR:                                   "A Windows Server has an incorrect configuration.",
	STATUS_FT_MISSING_MEMBER:                                           "An attempt was made to explicitly access the secondary copy of information via a device control to the fault tolerance driver and the secondary copy is not present in the system.",
	STATUS_ILL_FORMED_SERVICE_ENTRY:                                    "A configuration registry node that represents a driver service entry was ill-formed and did not contain the required value entries.",
	STATUS_ILLEGAL_CHARACTER:                                           "An illegal character was encountered. For a multibyte character set, this includes a lead byte without a succeeding trail byte. For the Unicode character set this includes the characters 0xFFFF and 0xFFFE.",
	STATUS_UNMAPPABLE_CHARACTER:                                        "No mapping for the Unicode character exists in the target multibyte code page.",
	STATUS_UNDEFINED_CHARACTER:                                         "The Unicode character is not defined in the Unicode character set that is installed on the system.",
	STATUS_FLOPPY_VOLUME:                                               "The paging file cannot be created on a floppy disk.",
	STATUS_FLOPPY_ID_MARK_NOT_FOUND:                                    "{Floppy Disk Error} While accessing a floppy disk, an ID address mark was not found.",
	STATUS_FLOPPY_WRONG_CYLINDER:                                       "{Floppy Disk Error} While accessing a floppy disk, the track address from the sector ID field was found to be different from the track address that is maintained by the controller.",
	STATUS_FLOPPY_UNKNOWN_ERROR:                                        "{Floppy Disk Error} The floppy disk controller reported an error that is not recognized by the floppy disk driver.",
	STATUS_FLOPPY_BAD_REGISTERS:                                        "{Floppy Disk Error} While accessing a floppy-disk, the controller returned inconsistent results via its registers.",
	STATUS_DISK_RECALIBRATE_FAILED:                                     "{Hard Disk Error} While accessing the hard disk, a recalibrate operation failed, even after retries.",
	STATUS_DISK_OPERATION_FAILED:                                       "{Hard Disk Error} While accessing the hard disk, a disk operation failed even after retries.",
	STATUS_DISK_RESET_FAILED:                                           "{Hard Disk Error} While accessing the hard disk, a disk controller reset was needed, but even that failed.",
	STATUS_SHARED_IRQ_BUSY:                                             "An attempt was made to open a device that was sharing an interrupt request (IRQ) with other devices. At least one other device that uses that IRQ was already opened. Two concurrent opens of devices that share an IRQ and only work via interrupts is not supported for the particular bus type that the devices use.",
	STATUS_FT_ORPHANING:                                                "{FT Orphaning} A disk that is part of a fault-tolerant volume can no longer be accessed.",
	STATUS_BIOS_FAILED_TO_CONNECT_INTERRUPT:                            "The basic input/output system (BIOS) failed to connect a system interrupt to the device or bus for which the device is connected.",
	STATUS_PARTITION_FAILURE:                                           "The tape could not be partitioned.",
	STATUS_INVALID_BLOCK_LENGTH:                                        "When accessing a new tape of a multi-volume partition, the current blocksize is incorrect.",
	STATUS_DEVICE_NOT_PARTITIONED:                                      "The tape partition information could not be found when loading a tape.",
	STATUS_UNABLE_TO_LOCK_MEDIA:                                        "An attempt to lock the eject media mechanism failed.",
	STATUS_UNABLE_TO_UNLOAD_MEDIA:                                      "An attempt to unload media failed.",
	STATUS_EOM_OVERFLOW:                                                "The physical end of tape was detected.",
	STATUS_NO_MEDIA:                                                    "{No Media} There is no media in the drive. Insert media into drive %hs.",
	STATUS_NO_SUCH_MEMBER:                                              "A member could not be added to or removed from the local group because the member does not exist.",
	STATUS_INVALID_MEMBER:                                              "A new member could not be added to a local group because the member has the wrong account type.",
	STATUS_KEY_DELETED:                                                 "An illegal operation was attempted on a registry key that has been marked for deletion.",
	STATUS_NO_LOG_SPACE:                                                "The system could not allocate the required space in a registry log.",
	STATUS_TOO_MANY_SIDS:                                               "Too many SIDs have been specified.",
	STATUS_LM_CROSS_ENCRYPTION_REQUIRED:                                "An attempt was made to change a user password in the security account manager without providing the necessary LM cross-encrypted password.",
	STATUS_KEY_HAS_CHILDREN:                                            "An attempt was made to create a symbolic link in a registry key that already has subkeys or values.",
	STATUS_CHILD_MUST_BE_VOLATILE:                                      "An attempt was made to create a stable subkey under a volatile parent key.",
	STATUS_DEVICE_CONFIGURATION_ERROR:                                  "The I/O device is configured incorrectly or the configuration parameters to the driver are incorrect.",
	STATUS_DRIVER_INTERNAL_ERROR:                                       "An error was detected between two drivers or within an I/O driver.",
	STATUS_INVALID_DEVICE_STATE:                                        "The device is not in a valid state to perform this request.",
	STATUS_IO_DEVICE_ERROR:                                             "The I/O device reported an I/O error.",
	STATUS_DEVICE_PROTOCOL_ERROR:                                       "A protocol error was detected between the driver and the device.",
	STATUS_BACKUP_CONTROLLER:                                           "This operation is only allowed for the primary domain controller of the domain.",
	STATUS_LOG_FILE_FULL:                                               "The log file space is insufficient to support this operation.",
	STATUS_TOO_LATE:                                                    "A write operation was attempted to a volume after it was dismounted.",
	STATUS_NO_TRUST_LSA_SECRET:                                         "The workstation does not have a trust secret for the primary domain in the local LSA database.",
	STATUS_NO_TRUST_SAM_ACCOUNT:                                        "The SAM database on the Windows Server does not have a computer account for this workstation trust relationship.",
	STATUS_TRUSTED_DOMAIN_FAILURE:                                      "The logon request failed because the trust relationship between the primary domain and the trusted domain failed.",
	STATUS_TRUSTED_RELATIONSHIP_FAILURE:                                "The logon request failed because the trust relationship between this workstation and the primary domain failed.",
	STATUS_EVENTLOG_FILE_CORRUPT:                                       "The Eventlog log file is corrupt.",
	STATUS_EVENTLOG_CANT_START:                                         "No Eventlog log file could be opened. The Eventlog service did not start.",
	STATUS_TRUST_FAILURE:                                               "The network logon failed. This may be because the validation authority cannot be reached.",
	STATUS_MUTANT_LIMIT_EXCEEDED:                                       "An attempt was made to acquire a mutant such that its maximum count would have been exceeded.",
	STATUS_NETLOGON_NOT_STARTED:                                        "An attempt was made to logon, but the NetLogon service was not started.",
	STATUS_ACCOUNT_EXPIRED:                                             "The user account has expired.",
	STATUS_POSSIBLE_DEADLOCK:                                           "{EXCEPTION} Possible deadlock condition.",
	STATUS_NETWORK_CREDENTIAL_CONFLICT:                                 "Multiple connections to a server or shared resource by the same user, using more than one user name, are not allowed. Disconnect all previous connections to the server or shared resource and try again.",
	STATUS_REMOTE_SESSION_LIMIT:                                        "An attempt was made to establish a session to a network server, but there are already too many sessions established to that server.",
	STATUS_EVENTLOG_FILE_CHANGED:                                       "The log file has changed between reads.",
	STATUS_NOLOGON_INTERDOMAIN_TRUST_ACCOUNT:                           "The account used is an interdomain trust account. Use your global user account or local user account to access this server.",
	STATUS_NOLOGON_WORKSTATION_TRUST_ACCOUNT:                           "The account used is a computer account. Use your global user account or local user account to access this server.",
	STATUS_NOLOGON_SERVER_TRUST_ACCOUNT:                                "The account used is a server trust account. Use your global user account or local user account to access this server.",
	STATUS_DOMAIN_TRUST_INCONSISTENT:                                   "The name or SID of the specified domain is inconsistent with the trust information for that domain.",
	STATUS_FS_DRIVER_REQUIRED:                                          "A volume has been accessed for which a file system driver is required that has not yet been loaded.",
	STATUS_IMAGE_ALREADY_LOADED_AS_DLL:                                 "Indicates that the specified image is already loaded as a DLL.",
	STATUS_INCOMPATIBLE_WITH_GLOBAL_SHORT_NAME_REGISTRY_SETTING:        "Short name settings may not be changed on this volume due to the global registry setting.",
	STATUS_SHORT_NAMES_NOT_ENABLED_ON_VOLUME:                           "Short names are not enabled on this volume.",
	STATUS_SECURITY_STREAM_IS_INCONSISTENT:                             "The security stream for the given volume is in an inconsistent state. Please run CHKDSK on the volume.",
	STATUS_INVALID_LOCK_RANGE:                                          "A requested file lock operation cannot be processed due to an invalid byte range.",
	STATUS_INVALID_ACE_CONDITION:                                       "The specified access control entry (ACE) contains an invalid condition.",
	STATUS_IMAGE_SUBSYSTEM_NOT_PRESENT:                                 "The subsystem needed to support the image type is not present.",
	STATUS_NOTIFICATION_GUID_ALREADY_DEFINED:                           "The specified file already has a notification GUID associated with it.",
	STATUS_NETWORK_OPEN_RESTRICTION:                                    "A remote open failed because the network open restrictions were not satisfied.",
	STATUS_NO_USER_SESSION_KEY:                                         "There is no user session key for the specified logon session.",
	STATUS_USER_SESSION_DELETED:                                        "The remote user session has been deleted.",
	STATUS_RESOURCE_LANG_NOT_FOUND:                                     "Indicates the specified resource language ID cannot be found in the image file.",
	STATUS_INSUFF_SERVER_RESOURCES:                                     "Insufficient server resources exist to complete the request.",
	STATUS_INVALID_BUFFER_SIZE:                                         "The size of the buffer is invalid for the specified operation.",
	STATUS_INVALID_ADDRESS_COMPONENT:                                   "The transport rejected the specified network address as invalid.",
	STATUS_INVALID_ADDRESS_WILDCARD:                                    "The transport rejected the specified network address due to invalid use of a wildcard.",
	STATUS_TOO_MANY_ADDRESSES:                                          "The transport address could not be opened because all the available addresses are in use.",
	STATUS_ADDRESS_ALREADY_EXISTS:                                      "The transport address could not be opened because it already exists.",
	STATUS_ADDRESS_CLOSED:                                              "The transport address is now closed.",
	STATUS_CONNECTION_DISCONNECTED:                                     "The transport connection is now disconnected.",
	STATUS_CONNECTION_RESET:                                            "The transport connection has been reset.",
	STATUS_TOO_MANY_NODES:                                              "The transport cannot dynamically acquire any more nodes.",
	STATUS_TRANSACTION_ABORTED:                                         "The transport aborted a pending transaction.",
	STATUS_TRANSACTION_TIMED_OUT:                                       "The transport timed out a request that is waiting for a response.",
	STATUS_TRANSACTION_NO_RELEASE:                                      "The transport did not receive a release for a pending response.",
	STATUS_TRANSACTION_NO_MATCH:                                        "The transport did not find a transaction that matches the specific token.",
	STATUS_TRANSACTION_RESPONDED:                                       "The transport had previously responded to a transaction request.",
	STATUS_TRANSACTION_INVALID_ID:                                      "The transport does not recognize the specified transaction request ID.",
	STATUS_TRANSACTION_INVALID_TYPE:                                    "The transport does not recognize the specified transaction request type.",
	STATUS_NOT_SERVER_SESSION:                                          "The transport can only process the specified request on the server side of a session.",
	STATUS_NOT_CLIENT_SESSION:                                          "The transport can only process the specified request on the client side of a session.",
	STATUS_CANNOT_LOAD_REGISTRY_FILE:                                   "{Registry File Failure} The registry cannot load the hive (file): %hs or its log or alternate. It is corrupt, absent, or not writable.",
	STATUS_DEBUG_ATTACH_FAILED:                                         "{Unexpected Failure in DebugActiveProcess} An unexpected failure occurred while processing a DebugActiveProcess API request. You may choose OK to terminate the process, or Cancel to ignore the error.",
	STATUS_SYSTEM_PROCESS_TERMINATED:                                   "{Fatal System Error} The %hs system process terminated unexpectedly with a status of 0x%08x (0x%08x 0x%08x). The system has been shut down.",
	STATUS_DATA_NOT_ACCEPTED:                                           "{Data Not Accepted} The TDI client could not handle the data received during an indication.",
	STATUS_NO_BROWSER_SERVERS_FOUND:                                    "{Unable to Retrieve Browser Server List} The list of servers for this workgroup is not currently available.",
	STATUS_VDM_HARD_ERROR:                                              "NTVDM encountered a hard error.",
	STATUS_DRIVER_CANCEL_TIMEOUT:                                       "{Cancel Timeout} The driver %hs failed to complete a canceled I/O request in the allotted time.",
	STATUS_REPLY_MESSAGE_MISMATCH:                                      "{Reply Message Mismatch} An attempt was made to reply to an LPC message, but the thread specified by the client ID in the message was not waiting on that message.",
	STATUS_MAPPED_ALIGNMENT:                                            "{Mapped View Alignment Incorrect} An attempt was made to map a view of a file, but either the specified base address or the offset into the file were not aligned on the proper allocation granularity.",
	STATUS_IMAGE_CHECKSUM_MISMATCH:                                     "{Bad Image Checksum} The image %hs is possibly corrupt. The header checksum does not match the computed checksum.",
	STATUS_LOST_WRITEBEHIND_DATA:                                       "{Delayed Write Failed} Windows was unable to save all the data for the file %hs. The data has been lost. This error may be caused by a failure of your computer hardware or network connection. Try to save this file elsewhere.",
	STATUS_CLIENT_SERVER_PARAMETERS_INVALID:                            "The parameters passed to the server in the client/server shared memory window were invalid. Too much data may have been put in the shared memory window.",
	STATUS_PASSWORD_MUST_CHANGE:                                        "The user password must be changed before logging on the first time.",
	STATUS_NOT_FOUND:                                                   "The object was not found.",
	STATUS_NOT_TINY_STREAM:                                             "The stream is not a tiny stream.",
	STATUS_RECOVERY_FAILURE:                                            "A transaction recovery failed.",
	STATUS_STACK_OVERFLOW_READ:                                         "The request must be handled by the stack overflow code.",
	STATUS_FAIL_CHECK:                                                  "A consistency check failed.",
	STATUS_DUPLICATE_OBJECTID:                                          "The attempt to insert the ID in the index failed because the ID is already in the index.",
	STATUS_OBJECTID_EXISTS:                                             "The attempt to set the object ID failed because the object already has an ID.",
	STATUS_CONVERT_TO_LARGE:                                            "Internal OFS status codes indicating how an allocation operation is handled. Either it is retried after the containing oNode is moved or the extent stream is converted to a large stream.",
	STATUS_RETRY:                                                       "The request needs to be retried.",
	STATUS_FOUND_OUT_OF_SCOPE:                                          "The attempt to find the object found an object on the volume that matches by ID; however, it is out of the scope of the handle that is used for the operation.",
	STATUS_ALLOCATE_BUCKET:                                             "The bucket array must be grown. Retry the transaction after doing so.",
	STATUS_PROPSET_NOT_FOUND:                                           "The specified property set does not exist on the object.",
	STATUS_MARSHALL_OVERFLOW:                                           "The user/kernel marshaling buffer has overflowed.",
	STATUS_INVALID_VARIANT:                                             "The supplied variant structure contains invalid data.",
	STATUS_DOMAIN_CONTROLLER_NOT_FOUND:                                 "A domain controller for this domain was not found.",
	STATUS_ACCOUNT_LOCKED_OUT:                                          "The user account has been automatically locked because too many invalid logon attempts or password change attempts have been requested.",
	STATUS_HANDLE_NOT_CLOSABLE:                                         "NtClose was called on a handle that was protected from close via NtSetInformationObject.",
	STATUS_CONNECTION_REFUSED:                                          "The transport-connection attempt was refused by the remote system.",
	STATUS_GRACEFUL_DISCONNECT:                                         "The transport connection was gracefully closed.",
	STATUS_ADDRESS_ALREADY_ASSOCIATED:                                  "The transport endpoint already has an address associated with it.",
	STATUS_ADDRESS_NOT_ASSOCIATED:                                      "An address has not yet been associated with the transport endpoint.",
	STATUS_CONNECTION_INVALID:                                          "An operation was attempted on a nonexistent transport connection.",
	STATUS_CONNECTION_ACTIVE:                                           "An invalid operation was attempted on an active transport connection.",
	STATUS_NETWORK_UNREACHABLE:                                         "The remote network is not reachable by the transport.",
	STATUS_HOST_UNREACHABLE:                                            "The remote system is not reachable by the transport.",
	STATUS_PROTOCOL_UNREACHABLE:                                        "The remote system does not support the transport protocol.",
	STATUS_PORT_UNREACHABLE:                                            "No service is operating at the destination port of the transport on the remote system.",
	STATUS_REQUEST_ABORTED:                                             "The request was aborted.",
	STATUS_CONNECTION_ABORTED:                                          "The transport connection was aborted by the local system.",
	STATUS_BAD_COMPRESSION_BUFFER:                                      "The specified buffer contains ill-formed data.",
	STATUS_USER_MAPPED_FILE:                                            "The requested operation cannot be performed on a file with a user mapped section open.",
	STATUS_AUDIT_FAILED:                                                "{Audit Failed} An attempt to generate a security audit failed.",
	STATUS_TIMER_RESOLUTION_NOT_SET:                                    "The timer resolution was not previously set by the current process.",
	STATUS_CONNECTION_COUNT_LIMIT:                                      "A connection to the server could not be made because the limit on the number of concurrent connections for this account has been reached.",
	STATUS_LOGIN_TIME_RESTRICTION:                                      "Attempting to log on during an unauthorized time of day for this account.",
	STATUS_LOGIN_WKSTA_RESTRICTION:                                     "The account is not authorized to log on from this station.",
	STATUS_IMAGE_MP_UP_MISMATCH:                                        "{UP/MP Image Mismatch} The image %hs has been modified for use on a uniprocessor system, but you are running it on a multiprocessor machine. Reinstall the image file.",
	STATUS_INSUFFICIENT_LOGON_INFO:                                     "There is insufficient account information to log you on.",
	STATUS_BAD_DLL_ENTRYPOINT:                                          "{Invalid DLL Entrypoint} The dynamic link library %hs is not written correctly. The stack pointer has been left in an inconsistent state. The entry point should be declared as WINAPI or STDCALL. Select YES to fail the DLL load. Select NO to continue execution. Selecting NO may cause the application to operate incorrectly.",
	STATUS_BAD_SERVICE_ENTRYPOINT:                                      "{Invalid Service Callback Entrypoint} The %hs service is not written correctly. The stack pointer has been left in an inconsistent state. The callback entry point should be declared as WINAPI or STDCALL. Selecting OK will cause the service to continue operation. However, the service process may operate incorrectly.",
	STATUS_LPC_REPLY_LOST:                                              "The server received the messages but did not send a reply.",
	STATUS_IP_ADDRESS_CONFLICT1:                                        "There is an IP address conflict with another system on the network.",
	STATUS_IP_ADDRESS_CONFLICT2:                                        "There is an IP address conflict with another system on the network.",
	STATUS_REGISTRY_QUOTA_LIMIT:                                        "{Low On Registry Space} The system has reached the maximum size that is allowed for the system part of the registry. Additional storage requests will be ignored.",
	STATUS_PATH_NOT_COVERED:                                            "The contacted server does not support the indicated part of the DFS namespace.",
	STATUS_NO_CALLBACK_ACTIVE:                                          "A callback return system service cannot be executed when no callback is active.",
	STATUS_LICENSE_QUOTA_EXCEEDED:                                      "The service being accessed is licensed for a particular number of connections. No more connections can be made to the service at this time because the service has already accepted the maximum number of connections.",
	STATUS_PWD_TOO_SHORT:                                               "The password provided is too short to meet the policy of your user account. Choose a longer password.",
	STATUS_PWD_TOO_RECENT:                                              "The policy of your user account does not allow you to change passwords too frequently. This is done to prevent users from changing back to a familiar, but potentially discovered, password. If you feel your password has been compromised, contact your administrator immediately to have a new one assigned.",
	STATUS_PWD_HISTORY_CONFLICT:                                        "You have attempted to change your password to one that you have used in the past. The policy of your user account does not allow this. Select a password that you have not previously used.",
	STATUS_PLUGPLAY_NO_DEVICE:                                          "You have attempted to load a legacy device driver while its device instance had been disabled.",
	STATUS_UNSUPPORTED_COMPRESSION:                                     "The specified compression format is unsupported.",
	STATUS_INVALID_HW_PROFILE:                                          "The specified hardware profile configuration is invalid.",
	STATUS_INVALID_PLUGPLAY_DEVICE_PATH:                                "The specified Plug and Play registry device path is invalid.",
	STATUS_DRIVER_ORDINAL_NOT_FOUND:                                    "{Driver Entry Point Not Found} The %hs device driver could not locate the ordinal %ld in driver %hs.",
	STATUS_DRIVER_ENTRYPOINT_NOT_FOUND:                                 "{Driver Entry Point Not Found} The %hs device driver could not locate the entry point %hs in driver %hs.",
	STATUS_RESOURCE_NOT_OWNED:                                          "{Application Error} The application attempted to release a resource it did not own. Click OK to terminate the application.",
	STATUS_TOO_MANY_LINKS:                                              "An attempt was made to create more links on a file than the file system supports.",
	STATUS_QUOTA_LIST_INCONSISTENT:                                     "The specified quota list is internally inconsistent with its descriptor.",
	STATUS_FILE_IS_OFFLINE:                                             "The specified file has been relocated to offline storage.",
	STATUS_EVALUATION_EXPIRATION:                                       "{Windows Evaluation Notification} The evaluation period for this installation of Windows has expired. This system will shutdown in 1 hour. To restore access to this installation of Windows, upgrade this installation by using a licensed distribution of this product.",
	STATUS_ILLEGAL_DLL_RELOCATION:                                      "{Illegal System DLL Relocation} The system DLL %hs was relocated in memory. The application will not run properly. The relocation occurred because the DLL %hs occupied an address range that is reserved for Windows system DLLs. The vendor supplying the DLL should be contacted for a new DLL.",
	STATUS_LICENSE_VIOLATION:                                           "{License Violation} The system has detected tampering with your registered product type. This is a violation of your software license. Tampering with the product type is not permitted.",
	STATUS_DLL_INIT_FAILED_LOGOFF:                                      "{DLL Initialization Failed} The application failed to initialize because the window station is shutting down.",
	STATUS_DRIVER_UNABLE_TO_LOAD:                                       "{Unable to Load Device Driver} %hs device driver could not be loaded. Error Status was 0x%x.",
	STATUS_DFS_UNAVAILABLE:                                             "DFS is unavailable on the contacted server.",
	STATUS_VOLUME_DISMOUNTED:                                           "An operation was attempted to a volume after it was dismounted.",
	STATUS_WX86_INTERNAL_ERROR:                                         "An internal error occurred in the Win32 x86 emulation subsystem.",
	STATUS_WX86_FLOAT_STACK_CHECK:                                      "Win32 x86 emulation subsystem floating-point stack check.",
	STATUS_VALIDATE_CONTINUE:                                           "The validation process needs to continue on to the next step.",
	STATUS_NO_MATCH:                                                    "There was no match for the specified key in the index.",
	STATUS_NO_MORE_MATCHES:                                             "There are no more matches for the current index enumeration.",
	STATUS_NOT_A_REPARSE_POINT:                                         "The NTFS file or directory is not a reparse point.",
	STATUS_IO_REPARSE_TAG_INVALID:                                      "The Windows I/O reparse tag passed for the NTFS reparse point is invalid.",
	STATUS_IO_REPARSE_TAG_MISMATCH:                                     "The Windows I/O reparse tag does not match the one that is in the NTFS reparse point.",
	STATUS_IO_REPARSE_DATA_INVALID:                                     "The user data passed for the NTFS reparse point is invalid.",
	STATUS_IO_REPARSE_TAG_NOT_HANDLED:                                  "The layered file system driver for this I/O tag did not handle it when needed.",
	STATUS_REPARSE_POINT_NOT_RESOLVED:                                  "The NTFS symbolic link could not be resolved even though the initial file name is valid.",
	STATUS_DIRECTORY_IS_A_REPARSE_POINT:                                "The NTFS directory is a reparse point.",
	STATUS_RANGE_LIST_CONFLICT:                                         "The range could not be added to the range list because of a conflict.",
	STATUS_SOURCE_ELEMENT_EMPTY:                                        "The specified medium changer source element contains no media.",
	STATUS_DESTINATION_ELEMENT_FULL:                                    "The specified medium changer destination element already contains media.",
	STATUS_ILLEGAL_ELEMENT_ADDRESS:                                     "The specified medium changer element does not exist.",
	STATUS_MAGAZINE_NOT_PRESENT:                                        "The specified element is contained in a magazine that is no longer present.",
	STATUS_REINITIALIZATION_NEEDED:                                     "The device requires re-initialization due to hardware errors.",
	STATUS_ENCRYPTION_FAILED:                                           "The file encryption attempt failed.",
	STATUS_DECRYPTION_FAILED:                                           "The file decryption attempt failed.",
	STATUS_RANGE_NOT_FOUND:                                             "The specified range could not be found in the range list.",
	STATUS_NO_RECOVERY_POLICY:                                          "There is no encryption recovery policy configured for this system.",
	STATUS_NO_EFS:                                                      "The required encryption driver is not loaded for this system.",
	STATUS_WRONG_EFS:                                                   "The file was encrypted with a different encryption driver than is currently loaded.",
	STATUS_NO_USER_KEYS:                                                "There are no EFS keys defined for the user.",
	STATUS_FILE_NOT_ENCRYPTED:                                          "The specified file is not encrypted.",
	STATUS_NOT_EXPORT_FORMAT:                                           "The specified file is not in the defined EFS export format.",
	STATUS_FILE_ENCRYPTED:                                              "The specified file is encrypted and the user does not have the ability to decrypt it.",
	STATUS_WMI_GUID_NOT_FOUND:                                          "The GUID passed was not recognized as valid by a WMI data provider.",
	STATUS_WMI_INSTANCE_NOT_FOUND:                                      "The instance name passed was not recognized as valid by a WMI data provider.",
	STATUS_WMI_ITEMID_NOT_FOUND:                                        "The data item ID passed was not recognized as valid by a WMI data provider.",
	STATUS_WMI_TRY_AGAIN:                                               "The WMI request could not be completed and should be retried.",
	STATUS_SHARED_POLICY:                                               "The policy object is shared and can only be modified at the root.",
	STATUS_POLICY_OBJECT_NOT_FOUND:                                     "The policy object does not exist when it should.",
	STATUS_POLICY_ONLY_IN_DS:                                           "The requested policy information only lives in the Ds.",
	STATUS_VOLUME_NOT_UPGRADED:                                         "The volume must be upgraded to enable this feature.",
	STATUS_REMOTE_STORAGE_NOT_ACTIVE:                                   "The remote storage service is not operational at this time.",
	STATUS_REMOTE_STORAGE_MEDIA_ERROR:                                  "The remote storage service encountered a media error.",
	STATUS_NO_TRACKING_SERVICE:                                         "The tracking (workstation) service is not running.",
	STATUS_SERVER_SID_MISMATCH:                                         "The server process is running under a SID that is different from the SID that is required by client.",
	STATUS_DS_NO_ATTRIBUTE_OR_VALUE:                                    "The specified directory service attribute or value does not exist.",
	STATUS_DS_INVALID_ATTRIBUTE_SYNTAX:                                 "The attribute syntax specified to the directory service is invalid.",
	STATUS_DS_ATTRIBUTE_TYPE_UNDEFINED:                                 "The attribute type specified to the directory service is not defined.",
	STATUS_DS_ATTRIBUTE_OR_VALUE_EXISTS:                                "The specified directory service attribute or value already exists.",
	STATUS_DS_BUSY:                                                     "The directory service is busy.",
	STATUS_DS_UNAVAILABLE:                                              "The directory service is unavailable.",
	STATUS_DS_NO_RIDS_ALLOCATED:                                        "The directory service was unable to allocate a relative identifier.",
	STATUS_DS_NO_MORE_RIDS:                                             "The directory service has exhausted the pool of relative identifiers.",
	STATUS_DS_INCORRECT_ROLE_OWNER:                                     "The requested operation could not be performed because the directory service is not the master for that type of operation.",
	STATUS_DS_RIDMGR_INIT_ERROR:                                        "The directory service was unable to initialize the subsystem that allocates relative identifiers.",
	STATUS_DS_OBJ_CLASS_VIOLATION:                                      "The requested operation did not satisfy one or more constraints that are associated with the class of the object.",
	STATUS_DS_CANT_ON_NON_LEAF:                                         "The directory service can perform the requested operation only on a leaf object.",
	STATUS_DS_CANT_ON_RDN:                                              "The directory service cannot perform the requested operation on the Relatively Defined Name (RDN) attribute of an object.",
	STATUS_DS_CANT_MOD_OBJ_CLASS:                                       "The directory service detected an attempt to modify the object class of an object.",
	STATUS_DS_CROSS_DOM_MOVE_FAILED:                                    "An error occurred while performing a cross domain move operation.",
	STATUS_DS_GC_NOT_AVAILABLE:                                         "Unable to contact the global catalog server.",
	STATUS_DIRECTORY_SERVICE_REQUIRED:                                  "The requested operation requires a directory service, and none was available.",
	STATUS_REPARSE_ATTRIBUTE_CONFLICT:                                  "The reparse attribute cannot be set because it is incompatible with an existing attribute.",
	STATUS_CANT_ENABLE_DENY_ONLY:                                       "A group marked \"use for deny only\" cannot be enabled.",
	STATUS_FLOAT_MULTIPLE_FAULTS:                                       "{EXCEPTION} Multiple floating-point faults.",
	STATUS_FLOAT_MULTIPLE_TRAPS:                                        "{EXCEPTION} Multiple floating-point traps.",
	STATUS_DEVICE_REMOVED:                                              "The device has been removed.",
	STATUS_JOURNAL_DELETE_IN_PROGRESS:                                  "The volume change journal is being deleted.",
	STATUS_JOURNAL_NOT_ACTIVE:                                          "The volume change journal is not active.",
	STATUS_NOINTERFACE:                                                 "The requested interface is not supported.",
	STATUS_DS_ADMIN_LIMIT_EXCEEDED:                                     "A directory service resource limit has been exceeded.",
	STATUS_DRIVER_FAILED_SLEEP:                                         "{System Standby Failed} The driver %hs does not support standby mode. Updating this driver may allow the system to go to standby mode.",
	STATUS_MUTUAL_AUTHENTICATION_FAILED:                                "Mutual Authentication failed. The server password is out of date at the domain controller.",
	STATUS_CORRUPT_SYSTEM_FILE:                                         "The system file %1 has become corrupt and has been replaced.",
	STATUS_DATATYPE_MISALIGNMENT_ERROR:                                 "{EXCEPTION} Alignment Error A data type misalignment error was detected in a load or store instruction.",
	STATUS_WMI_READ_ONLY:                                               "The WMI data item or data block is read-only.",
	STATUS_WMI_SET_FAILURE:                                             "The WMI data item or data block could not be changed.",
	STATUS_COMMITMENT_MINIMUM:                                          "{Virtual Memory Minimum Too Low} Your system is low on virtual memory. Windows is increasing the size of your virtual memory paging file. During this process, memory requests for some applications may be denied. For more information, see Help.",
	STATUS_REG_NAT_CONSUMPTION:                                         "{EXCEPTION} Register NaT consumption faults. A NaT value is consumed on a non-speculative instruction.",
	STATUS_TRANSPORT_FULL:                                              "The transport element of the medium changer contains media, which is causing the operation to fail.",
	STATUS_DS_SAM_INIT_FAILURE:                                         "Security Accounts Manager initialization failed because of the following error: %hs Error Status: 0x%x. Click OK to shut down this system and restart in Directory Services Restore Mode. Check the event log for more detailed information.",
	STATUS_ONLY_IF_CONNECTED:                                           "This operation is supported only when you are connected to the server.",
	STATUS_DS_SENSITIVE_GROUP_VIOLATION:                                "Only an administrator can modify the membership list of an administrative group.",
	STATUS_PNP_RESTART_ENUMERATION:                                     "A device was removed so enumeration must be restarted.",
	STATUS_JOURNAL_ENTRY_DELETED:                                       "The journal entry has been deleted from the journal.",
	STATUS_DS_CANT_MOD_PRIMARYGROUPID:                                  "Cannot change the primary group ID of a domain controller account.",
	STATUS_SYSTEM_IMAGE_BAD_SIGNATURE:                                  "{Fatal System Error} The system image %s is not properly signed. The file has been replaced with the signed file. The system has been shut down.",
	STATUS_PNP_REBOOT_REQUIRED:                                         "The device will not start without a reboot.",
	STATUS_POWER_STATE_INVALID:                                         "The power state of the current device cannot support this request.",
	STATUS_DS_INVALID_GROUP_TYPE:                                       "The specified group type is invalid.",
	STATUS_DS_NO_NEST_GLOBALGROUP_IN_MIXEDDOMAIN:                       "In a mixed domain, no nesting of a global group if the group is security enabled.",
	STATUS_DS_NO_NEST_LOCALGROUP_IN_MIXEDDOMAIN:                        "In a mixed domain, cannot nest local groups with other local groups, if the group is security enabled.",
	STATUS_DS_GLOBAL_CANT_HAVE_LOCAL_MEMBER:                            "A global group cannot have a local group as a member.",
	STATUS_DS_GLOBAL_CANT_HAVE_UNIVERSAL_MEMBER:                        "A global group cannot have a universal group as a member.",
	STATUS_DS_UNIVERSAL_CANT_HAVE_LOCAL_MEMBER:                         "A universal group cannot have a local group as a member.",
	STATUS_DS_GLOBAL_CANT_HAVE_CROSSDOMAIN_MEMBER:                      "A global group cannot have a cross-domain member.",
	STATUS_DS_LOCAL_CANT_HAVE_CROSSDOMAIN_LOCAL_MEMBER:                 "A local group cannot have another cross-domain local group as a member.",
	STATUS_DS_HAVE_PRIMARY_MEMBERS:                                     "Cannot change to a security-disabled group because primary members are in this group.",
	STATUS_WMI_NOT_SUPPORTED:                                           "The WMI operation is not supported by the data block or method.",
	STATUS_INSUFFICIENT_POWER:                                          "There is not enough power to complete the requested operation.",
	STATUS_SAM_NEED_BOOTKEY_PASSWORD:                                   "The Security Accounts Manager needs to get the boot password.",
	STATUS_SAM_NEED_BOOTKEY_FLOPPY:                                     "The Security Accounts Manager needs to get the boot key from the floppy disk.",
	STATUS_DS_CANT_START:                                               "The directory service cannot start.",
	STATUS_DS_INIT_FAILURE:                                             "The directory service could not start because of the following error: %hs Error Status: 0x%x. Click OK to shut down this system and restart in Directory Services Restore Mode. Check the event log for more detailed information.",
	STATUS_SAM_INIT_FAILURE:                                            "The Security Accounts Manager initialization failed because of the following error: %hs Error Status: 0x%x. Click OK to shut down this system and restart in Safe Mode. Check the event log for more detailed information.",
	STATUS_DS_GC_REQUIRED:                                              "The requested operation can be performed only on a global catalog server.",
	STATUS_DS_LOCAL_MEMBER_OF_LOCAL_ONLY:                               "A local group can only be a member of other local groups in the same domain.",
	STATUS_DS_NO_FPO_IN_UNIVERSAL_GROUPS:                               "Foreign security principals cannot be members of universal groups.",
	STATUS_DS_MACHINE_ACCOUNT_QUOTA_EXCEEDED:                           "Your computer could not be joined to the domain. You have exceeded the maximum number of computer accounts you are allowed to create in this domain. Contact your system administrator to have this limit reset or increased.",
	STATUS_CURRENT_DOMAIN_NOT_ALLOWED:                                  "This operation cannot be performed on the current domain.",
	STATUS_CANNOT_MAKE:                                                 "The directory or file cannot be created.",
	STATUS_SYSTEM_SHUTDOWN:                                             "The system is in the process of shutting down.",
	STATUS_DS_INIT_FAILURE_CONSOLE:                                     "Directory Services could not start because of the following error: %hs Error Status: 0x%x. Click OK to shut down the system. You can use the recovery console to diagnose the system further.",
	STATUS_DS_SAM_INIT_FAILURE_CONSOLE:                                 "Security Accounts Manager initialization failed because of the following error: %hs Error Status: 0x%x. Click OK to shut down the system. You can use the recovery console to diagnose the system further.",
	STATUS_UNFINISHED_CONTEXT_DELETED:                                  "A security context was deleted before the context was completed. This is considered a logon failure.",
	STATUS_NO_TGT_REPLY:                                                "The client is trying to negotiate a context and the server requires user-to-user but did not send a TGT reply.",
	STATUS_OBJECTID_NOT_FOUND:                                          "An object ID was not found in the file.",
	STATUS_NO_IP_ADDRESSES:                                             "Unable to accomplish the requested task because the local machine does not have any IP addresses.",
	STATUS_WRONG_CREDENTIAL_HANDLE:                                     "The supplied credential handle does not match the credential that is associated with the security context.",
	STATUS_CRYPTO_SYSTEM_INVALID:                                       "The crypto system or checksum function is invalid because a required function is unavailable.",
	STATUS_MAX_REFERRALS_EXCEEDED:                                      "The number of maximum ticket referrals has been exceeded.",
	STATUS_MUST_BE_KDC:                                                 "The local machine must be a Kerberos KDC (domain controller) and it is not.",
	STATUS_STRONG_CRYPTO_NOT_SUPPORTED:                                 "The other end of the security negotiation requires strong crypto but it is not supported on the local machine.",
	STATUS_TOO_MANY_PRINCIPALS:                                         "The KDC reply contained more than one principal name.",
	STATUS_NO_PA_DATA:                                                  "Expected to find PA data for a hint of what etype to use, but it was not found.",
	STATUS_PKINIT_NAME_MISMATCH:                                        "The client certificate does not contain a valid UPN, or does not match the client name in the logon request. Contact your administrator.",
	STATUS_SMARTCARD_LOGON_REQUIRED:                                    "Smart card logon is required and was not used.",
	STATUS_KDC_INVALID_REQUEST:                                         "An invalid request was sent to the KDC.",
	STATUS_KDC_UNABLE_TO_REFER:                                         "The KDC was unable to generate a referral for the service requested.",
	STATUS_KDC_UNKNOWN_ETYPE:                                           "The encryption type requested is not supported by the KDC.",
	STATUS_SHUTDOWN_IN_PROGRESS:                                        "A system shutdown is in progress.",
	STATUS_SERVER_SHUTDOWN_IN_PROGRESS:                                 "The server machine is shutting down.",
	STATUS_NOT_SUPPORTED_ON_SBS:                                        "This operation is not supported on a computer running Windows Server 2003 operating system for Small Business Server.",
	STATUS_WMI_GUID_DISCONNECTED:                                       "The WMI GUID is no longer available.",
	STATUS_WMI_ALREADY_DISABLED:                                        "Collection or events for the WMI GUID is already disabled.",
	STATUS_WMI_ALREADY_ENABLED:                                         "Collection or events for the WMI GUID is already enabled.",
	STATUS_MFT_TOO_FRAGMENTED:                                          "The master file table on the volume is too fragmented to complete this operation.",
	STATUS_COPY_PROTECTION_FAILURE:                                     "Copy protection failure.",
	STATUS_CSS_AUTHENTICATION_FAILURE:                                  "Copy protection error—DVD CSS Authentication failed.",
	STATUS_CSS_KEY_NOT_PRESENT:                                         "Copy protection error—The specified sector does not contain a valid key.",
	STATUS_CSS_KEY_NOT_ESTABLISHED:                                     "Copy protection error—DVD session key not established.",
	STATUS_CSS_SCRAMBLED_SECTOR:                                        "Copy protection error—The read failed because the sector is encrypted.",
	STATUS_CSS_REGION_MISMATCH:                                         "Copy protection error—The region of the specified DVD does not correspond to the region setting of the drive.",
	STATUS_CSS_RESETS_EXHAUSTED:                                        "Copy protection error—The region setting of the drive may be permanent.",
	STATUS_PKINIT_FAILURE:                                              "The Kerberos protocol encountered an error while validating the KDC certificate during smart card logon. There is more information in the system event log.",
	STATUS_SMARTCARD_SUBSYSTEM_FAILURE:                                 "The Kerberos protocol encountered an error while attempting to use the smart card subsystem.",
	STATUS_NO_KERB_KEY:                                                 "The target server does not have acceptable Kerberos credentials.",
	STATUS_HOST_DOWN:                                                   "The transport determined that the remote system is down.",
	STATUS_UNSUPPORTED_PREAUTH:                                         "An unsupported pre-authentication mechanism was presented to the Kerberos package.",
	STATUS_EFS_ALG_BLOB_TOO_BIG:                                        "The encryption algorithm that is used on the source file needs a bigger key buffer than the one that is used on the destination file.",
	STATUS_PORT_NOT_SET:                                                "An attempt to remove a processes DebugPort was made, but a port was not already associated with the process.",
	STATUS_DEBUGGER_INACTIVE:                                           "An attempt to do an operation on a debug port failed because the port is in the process of being deleted.",
	STATUS_DS_VERSION_CHECK_FAILURE:                                    "This version of Windows is not compatible with the behavior version of the directory forest, domain, or domain controller.",
	STATUS_AUDITING_DISABLED:                                           "The specified event is currently not being audited.",
	STATUS_PRENT4_MACHINE_ACCOUNT:                                      "The machine account was created prior to Windows NT 4.0 operating system. The account needs to be recreated.",
	STATUS_DS_AG_CANT_HAVE_UNIVERSAL_MEMBER:                            "An account group cannot have a universal group as a member.",
	STATUS_INVALID_IMAGE_WIN_32:                                        "The specified image file did not have the correct format; it appears to be a 32-bit Windows image.",
	STATUS_INVALID_IMAGE_WIN_64:                                        "The specified image file did not have the correct format; it appears to be a 64-bit Windows image.",
	STATUS_BAD_BINDINGS:                                                "The client's supplied SSPI channel bindings were incorrect.",
	STATUS_NETWORK_SESSION_EXPIRED:                                     "The client session has expired; so the client must re-authenticate to continue accessing the remote resources.",
	STATUS_APPHELP_BLOCK:                                               "The AppHelp dialog box canceled; thus preventing the application from starting.",
	STATUS_ALL_SIDS_FILTERED:                                           "The SID filtering operation removed all SIDs.",
	STATUS_NOT_SAFE_MODE_DRIVER:                                        "The driver was not loaded because the system is starting in safe mode.",
	STATUS_ACCESS_DISABLED_BY_POLICY_DEFAULT:                           "Access to %1 has been restricted by your Administrator by the default software restriction policy level.",
	STATUS_ACCESS_DISABLED_BY_POLICY_PATH:                              "Access to %1 has been restricted by your Administrator by location with policy rule %2 placed on path %3.",
	STATUS_ACCESS_DISABLED_BY_POLICY_PUBLISHER:                         "Access to %1 has been restricted by your Administrator by software publisher policy.",
	STATUS_ACCESS_DISABLED_BY_POLICY_OTHER:                             "Access to %1 has been restricted by your Administrator by policy rule %2.",
	STATUS_FAILED_DRIVER_ENTRY:                                         "The driver was not loaded because it failed its initialization call.",
	STATUS_DEVICE_ENUMERATION_ERROR:                                    "The device encountered an error while applying power or reading the device configuration. This may be caused by a failure of your hardware or by a poor connection.",
	STATUS_MOUNT_POINT_NOT_RESOLVED:                                    "The create operation failed because the name contained at least one mount point that resolves to a volume to which the specified device object is not attached.",
	STATUS_INVALID_DEVICE_OBJECT_PARAMETER:                             "The device object parameter is either not a valid device object or is not attached to the volume that is specified by the file name.",
	STATUS_MCA_OCCURED:                                                 "A machine check error has occurred. Check the system event log for additional information.",
	STATUS_DRIVER_BLOCKED_CRITICAL:                                     "Driver %2 has been blocked from loading.",
	STATUS_DRIVER_BLOCKED:                                              "Driver %2 has been blocked from loading.",
	STATUS_DRIVER_DATABASE_ERROR:                                       "There was error [%2] processing the driver database.",
	STATUS_SYSTEM_HIVE_TOO_LARGE:                                       "System hive size has exceeded its limit.",
	STATUS_INVALID_IMPORT_OF_NON_DLL:                                   "A dynamic link library (DLL) referenced a module that was neither a DLL nor the process's executable image.",
	STATUS_NO_SECRETS:                                                  "The local account store does not contain secret material for the specified account.",
	STATUS_ACCESS_DISABLED_NO_SAFER_UI_BY_POLICY:                       "Access to %1 has been restricted by your Administrator by policy rule %2.",
	STATUS_FAILED_STACK_SWITCH:                                         "The system was not able to allocate enough memory to perform a stack switch.",
	STATUS_HEAP_CORRUPTION:                                             "A heap has been corrupted.",
	STATUS_SMARTCARD_WRONG_PIN:                                         "An incorrect PIN was presented to the smart card.",
	STATUS_SMARTCARD_CARD_BLOCKED:                                      "The smart card is blocked.",
	STATUS_SMARTCARD_CARD_NOT_AUTHENTICATED:                            "No PIN was presented to the smart card.",
	STATUS_SMARTCARD_NO_CARD:                                           "No smart card is available.",
	STATUS_SMARTCARD_NO_KEY_CONTAINER:                                  "The requested key container does not exist on the smart card.",
	STATUS_SMARTCARD_NO_CERTIFICATE:                                    "The requested certificate does not exist on the smart card.",
	STATUS_SMARTCARD_NO_KEYSET:                                         "The requested keyset does not exist.",
	STATUS_SMARTCARD_IO_ERROR:                                          "A communication error with the smart card has been detected.",
	STATUS_DOWNGRADE_DETECTED:                                          "The system detected a possible attempt to compromise security. Ensure that you can contact the server that authenticated you.",
	STATUS_SMARTCARD_CERT_REVOKED:                                      "The smart card certificate used for authentication has been revoked. Contact your system administrator. There may be additional information in the event log.",
	STATUS_ISSUING_CA_UNTRUSTED:                                        "An untrusted certificate authority was detected while processing the smart card certificate that is used for authentication. Contact your system administrator.",
	STATUS_REVOCATION_OFFLINE_C:                                        "The revocation status of the smart card certificate that is used for authentication could not be determined. Contact your system administrator.",
	STATUS_PKINIT_CLIENT_FAILURE:                                       "The smart card certificate used for authentication was not trusted. Contact your system administrator.",
	STATUS_SMARTCARD_CERT_EXPIRED:                                      "The smart card certificate used for authentication has expired. Contact your system administrator.",
	STATUS_DRIVER_FAILED_PRIOR_UNLOAD:                                  "The driver could not be loaded because a previous version of the driver is still in memory.",
	STATUS_SMARTCARD_SILENT_CONTEXT:                                    "The smart card provider could not perform the action because the context was acquired as silent.",
	STATUS_PER_USER_TRUST_QUOTA_EXCEEDED:                               "The delegated trust creation quota of the current user has been exceeded.",
	STATUS_ALL_USER_TRUST_QUOTA_EXCEEDED:                               "The total delegated trust creation quota has been exceeded.",
	STATUS_USER_DELETE_TRUST_QUOTA_EXCEEDED:                            "The delegated trust deletion quota of the current user has been exceeded.",
	STATUS_DS_NAME_NOT_UNIQUE:                                          "The requested name already exists as a unique identifier.",
	STATUS_DS_DUPLICATE_ID_FOUND:                                       "The requested object has a non-unique identifier and cannot be retrieved.",
	STATUS_DS_GROUP_CONVERSION_ERROR:                                   "The group cannot be converted due to attribute restrictions on the requested group type.",
	STATUS_VOLSNAP_PREPARE_HIBERNATE:                                   "{Volume Shadow Copy Service} Wait while the Volume Shadow Copy Service prepares volume %hs for hibernation.",
	STATUS_USER2USER_REQUIRED:                                          "Kerberos sub-protocol User2User is required.",
	STATUS_STACK_BUFFER_OVERRUN:                                        "The system detected an overrun of a stack-based buffer in this application. This overrun could potentially allow a malicious user to gain control of this application.",
	STATUS_NO_S4U_PROT_SUPPORT:                                         "The Kerberos subsystem encountered an error. A service for user protocol request was made against a domain controller which does not support service for user.",
	STATUS_CROSSREALM_DELEGATION_FAILURE:                               "An attempt was made by this server to make a Kerberos constrained delegation request for a target that is outside the server realm. This action is not supported and the resulting error indicates a misconfiguration on the allowed-to-delegate-to list for this server. Contact your administrator.",
	STATUS_REVOCATION_OFFLINE_KDC:                                      "The revocation status of the domain controller certificate used for smart card authentication could not be determined. There is additional information in the system event log. Contact your system administrator.",
	STATUS_ISSUING_CA_UNTRUSTED_KDC:                                    "An untrusted certificate authority was detected while processing the domain controller certificate used for authentication. There is additional information in the system event log. Contact your system administrator.",
	STATUS_KDC_CERT_EXPIRED:                                            "The domain controller certificate used for smart card logon has expired. Contact your system administrator with the contents of your system event log.",
	STATUS_KDC_CERT_REVOKED:                                            "The domain controller certificate used for smart card logon has been revoked. Contact your system administrator with the contents of your system event log.",
	STATUS_PARAMETER_QUOTA_EXCEEDED:                                    "Data present in one of the parameters is more than the function can operate on.",
	STATUS_HIBERNATION_FAILURE:                                         "The system has failed to hibernate (The error code is %hs). Hibernation will be disabled until the system is restarted.",
	STATUS_DELAY_LOAD_FAILED:                                           "An attempt to delay-load a .dll or get a function address in a delay-loaded .dll failed.",
	STATUS_AUTHENTICATION_FIREWALL_FAILED:                              "Logon Failure: The machine you are logging onto is protected by an authentication firewall. The specified account is not allowed to authenticate to the machine.",
	STATUS_VDM_DISALLOWED:                                              "%hs is a 16-bit application. You do not have permissions to execute 16-bit applications. Check your permissions with your system administrator.",
	STATUS_HUNG_DISPLAY_DRIVER_THREAD:                                  "{Display Driver Stopped Responding} The %hs display driver has stopped working normally. Save your work and reboot the system to restore full display functionality. The next time you reboot the machine a dialog will be displayed giving you a chance to report this failure to Microsoft.",
	STATUS_INSUFFICIENT_RESOURCE_FOR_SPECIFIED_SHARED_SECTION_SIZE:     "The Desktop heap encountered an error while allocating session memory. There is more information in the system event log.",
	STATUS_INVALID_CRUNTIME_PARAMETER:                                  "An invalid parameter was passed to a C runtime function.",
	STATUS_NTLM_BLOCKED:                                                "The authentication failed because NTLM was blocked.",
	STATUS_DS_SRC_SID_EXISTS_IN_FOREST:                                 "The source object's SID already exists in destination forest.",
	STATUS_DS_DOMAIN_NAME_EXISTS_IN_FOREST:                             "The domain name of the trusted domain already exists in the forest.",
	STATUS_DS_FLAT_NAME_EXISTS_IN_FOREST:                               "The flat name of the trusted domain already exists in the forest.",
	STATUS_INVALID_USER_PRINCIPAL_NAME:                                 "The User Principal Name (UPN) is invalid.",
	STATUS_ASSERTION_FAILURE:                                           "There has been an assertion failure.",
	STATUS_VERIFIER_STOP:                                               "Application verifier has found an error in the current process.",
	STATUS_CALLBACK_POP_STACK:                                          "A user mode unwind is in progress.",
	STATUS_INCOMPATIBLE_DRIVER_BLOCKED:                                 "%2 has been blocked from loading due to incompatibility with this system. Contact your software vendor for a compatible version of the driver.",
	STATUS_HIVE_UNLOADED:                                               "Illegal operation attempted on a registry key which has already been unloaded.",
	STATUS_COMPRESSION_DISABLED:                                        "Compression is disabled for this volume.",
	STATUS_FILE_SYSTEM_LIMITATION:                                      "The requested operation could not be completed due to a file system limitation.",
	STATUS_INVALID_IMAGE_HASH:                                          "The hash for image %hs cannot be found in the system catalogs. The image is likely corrupt or the victim of tampering.",
	STATUS_NOT_CAPABLE:                                                 "The implementation is not capable of performing the request.",
	STATUS_REQUEST_OUT_OF_SEQUENCE:                                     "The requested operation is out of order with respect to other operations.",
	STATUS_IMPLEMENTATION_LIMIT:                                        "An operation attempted to exceed an implementation-defined limit.",
	STATUS_ELEVATION_REQUIRED:                                          "The requested operation requires elevation.",
	STATUS_NO_SECURITY_CONTEXT:                                         "The required security context does not exist.",
	STATUS_PKU2U_CERT_FAILURE:                                          "The PKU2U protocol encountered an error while attempting to utilize the associated certificates.",
	STATUS_BEYOND_VDL:                                                  "The operation was attempted beyond the valid data length of the file.",
	STATUS_ENCOUNTERED_WRITE_IN_PROGRESS:                               "The attempted write operation encountered a write already in progress for some portion of the range.",
	STATUS_PTE_CHANGED:                                                 "The page fault mappings changed in the middle of processing a fault so the operation must be retried.",
	STATUS_PURGE_FAILED:                                                "The attempt to purge this file from memory failed to purge some or all the data from memory.",
	STATUS_CRED_REQUIRES_CONFIRMATION:                                  "The requested credential requires confirmation.",
	STATUS_CS_ENCRYPTION_INVALID_SERVER_RESPONSE:                       "The remote server sent an invalid response for a file being opened with Client Side Encryption.",
	STATUS_CS_ENCRYPTION_UNSUPPORTED_SERVER:                            "Client Side Encryption is not supported by the remote server even though it claims to support it.",
	STATUS_CS_ENCRYPTION_EXISTING_ENCRYPTED_FILE:                       "File is encrypted and should be opened in Client Side Encryption mode.",
	STATUS_CS_ENCRYPTION_NEW_ENCRYPTED_FILE:                            "A new encrypted file is being created and a $EFS needs to be provided.",
	STATUS_CS_ENCRYPTION_FILE_NOT_CSE:                                  "The SMB client requested a CSE FSCTL on a non-CSE file.",
	STATUS_INVALID_LABEL:                                               "Indicates a particular Security ID may not be assigned as the label of an object.",
	STATUS_DRIVER_PROCESS_TERMINATED:                                   "The process hosting the driver for this device has terminated.",
	STATUS_AMBIGUOUS_SYSTEM_DEVICE:                                     "The requested system device cannot be identified due to multiple indistinguishable devices potentially matching the identification criteria.",
	STATUS_SYSTEM_DEVICE_NOT_FOUND:                                     "The requested system device cannot be found.",
	STATUS_RESTART_BOOT_APPLICATION:                                    "This boot application must be restarted.",
	STATUS_INSUFFICIENT_NVRAM_RESOURCES:                                "Insufficient NVRAM resources exist to complete the API.\u00a0 A reboot might be required.",
	STATUS_NO_RANGES_PROCESSED:                                         "No ranges for the specified operation were able to be processed.",
	STATUS_DEVICE_FEATURE_NOT_SUPPORTED:                                "The storage device does not support Offload Write.",
	STATUS_DEVICE_UNREACHABLE:                                          "Data cannot be moved because the source device cannot communicate with the destination device.",
	STATUS_INVALID_TOKEN:                                               "The token representing the data is invalid or expired.",
	STATUS_SERVER_UNAVAILABLE:                                          "The file server is temporarily unavailable.",
	STATUS_INVALID_TASK_NAME:                                           "The specified task name is invalid.",
	STATUS_INVALID_TASK_INDEX:                                          "The specified task index is invalid.",
	STATUS_THREAD_ALREADY_IN_TASK:                                      "The specified thread is already joining a task.",
	STATUS_CALLBACK_BYPASS:                                             "A callback has requested to bypass native code.",
	STATUS_FAIL_FAST_EXCEPTION:                                         "A fail fast exception occurred. Exception handlers will not be invoked and the process will be terminated immediately.",
	STATUS_IMAGE_CERT_REVOKED:                                          "Windows cannot verify the digital signature for this file. The signing certificate for this file has been revoked.",
	STATUS_PORT_CLOSED:                                                 "The ALPC port is closed.",
	STATUS_MESSAGE_LOST:                                                "The ALPC message requested is no longer available.",
	STATUS_INVALID_MESSAGE:                                             "The ALPC message supplied is invalid.",
	STATUS_REQUEST_CANCELED:                                            "The ALPC message has been canceled.",
	STATUS_RECURSIVE_DISPATCH:                                          "Invalid recursive dispatch attempt.",
	STATUS_LPC_RECEIVE_BUFFER_EXPECTED:                                 "No receive buffer has been supplied in a synchronous request.",
	STATUS_LPC_INVALID_CONNECTION_USAGE:                                "The connection port is used in an invalid context.",
	STATUS_LPC_REQUESTS_NOT_ALLOWED:                                    "The ALPC port does not accept new request messages.",
	STATUS_RESOURCE_IN_USE:                                             "The resource requested is already in use.",
	STATUS_HARDWARE_MEMORY_ERROR:                                       "The hardware has reported an uncorrectable memory error.",
	STATUS_THREADPOOL_HANDLE_EXCEPTION:                                 "Status 0x%08x was returned, waiting on handle 0x%x for wait 0x%p, in waiter 0x%p.",
	STATUS_THREADPOOL_SET_EVENT_ON_COMPLETION_FAILED:                   "After a callback to 0x%p(0x%p), a completion call to Set event(0x%p) failed with status 0x%08x.",
	STATUS_THREADPOOL_RELEASE_SEMAPHORE_ON_COMPLETION_FAILED:           "After a callback to 0x%p(0x%p), a completion call to ReleaseSemaphore(0x%p, %d) failed with status 0x%08x.",
	STATUS_THREADPOOL_RELEASE_MUTEX_ON_COMPLETION_FAILED:               "After a callback to 0x%p(0x%p), a completion call to ReleaseMutex(%p) failed with status 0x%08x.",
	STATUS_THREADPOOL_FREE_LIBRARY_ON_COMPLETION_FAILED:                "After a callback to 0x%p(0x%p), a completion call to FreeLibrary(%p) failed with status 0x%08x.",
	STATUS_THREADPOOL_RELEASED_DURING_OPERATION:                        "The thread pool 0x%p was released while a thread was posting a callback to 0x%p(0x%p) to it.",
	STATUS_CALLBACK_RETURNED_WHILE_IMPERSONATING:                       "A thread pool worker thread is impersonating a client, after a callback to 0x%p(0x%p). This is unexpected, indicating that the callback is missing a call to revert the impersonation.",
	STATUS_APC_RETURNED_WHILE_IMPERSONATING:                            "A thread pool worker thread is impersonating a client, after executing an APC. This is unexpected, indicating that the APC is missing a call to revert the impersonation.",
	STATUS_PROCESS_IS_PROTECTED:                                        "Either the target process, or the target thread's containing process, is a protected process.",
	STATUS_MCA_EXCEPTION:                                               "A thread is getting dispatched with MCA EXCEPTION because of MCA.",
	STATUS_CERTIFICATE_MAPPING_NOT_UNIQUE:                              "The client certificate account mapping is not unique.",
	STATUS_SYMLINK_CLASS_DISABLED:                                      "The symbolic link cannot be followed because its type is disabled.",
	STATUS_INVALID_IDN_NORMALIZATION:                                   "Indicates that the specified string is not valid for IDN normalization.",
	STATUS_NO_UNICODE_TRANSLATION:                                      "No mapping for the Unicode character exists in the target multi-byte code page.",
	STATUS_ALREADY_REGISTERED:                                          "The provided callback is already registered.",
	STATUS_CONTEXT_MISMATCH:                                            "The provided context did not match the target.",
	STATUS_PORT_ALREADY_HAS_COMPLETION_LIST:                            "The specified port already has a completion list.",
	STATUS_CALLBACK_RETURNED_THREAD_PRIORITY:                           "A threadpool worker thread entered a callback at thread base priority 0x%x and exited at priority 0x%x.This is unexpected, indicating that the callback missed restoring the priority.",
	STATUS_INVALID_THREAD:                                              "An invalid thread, handle %p, is specified for this operation. Possibly, a threadpool worker thread was specified.",
	STATUS_CALLBACK_RETURNED_TRANSACTION:                               "A threadpool worker thread entered a callback, which left transaction state.This is unexpected, indicating that the callback missed clearing the transaction.",
	STATUS_CALLBACK_RETURNED_LDR_LOCK:                                  "A threadpool worker thread entered a callback, which left the loader lock held.This is unexpected, indicating that the callback missed releasing the lock.",
	STATUS_CALLBACK_RETURNED_LANG:                                      "A threadpool worker thread entered a callback, which left with preferred languages set.This is unexpected, indicating that the callback missed clearing them.",
	STATUS_CALLBACK_RETURNED_PRI_BACK:                                  "A threadpool worker thread entered a callback, which left with background priorities set.This is unexpected, indicating that the callback missed restoring the original priorities.",
	STATUS_DISK_REPAIR_DISABLED:                                        "The attempted operation required self healing to be enabled.",
	STATUS_DS_DOMAIN_RENAME_IN_PROGRESS:                                "The directory service cannot perform the requested operation because a domain rename operation is in progress.",
	STATUS_DISK_QUOTA_EXCEEDED:                                         "An operation failed because the storage quota was exceeded.",
	STATUS_CONTENT_BLOCKED:                                             "An operation failed because the content was blocked.",
	STATUS_BAD_CLUSTERS:                                                "The operation could not be completed due to bad clusters on disk.",
	STATUS_VOLUME_DIRTY:                                                "The operation could not be completed because the volume is dirty. Please run the Chkdsk utility and try again. ",
	STATUS_FILE_CHECKED_OUT:                                            "This file is checked out or locked for editing by another user.",
	STATUS_CHECKOUT_REQUIRED:                                           "The file must be checked out before saving changes.",
	STATUS_BAD_FILE_TYPE:                                               "The file type being saved or retrieved has been blocked.",
	STATUS_FILE_TOO_LARGE:                                              "The file size exceeds the limit allowed and cannot be saved.",
	STATUS_FORMS_AUTH_REQUIRED:                                         "Access Denied. Before opening files in this location, you must first browse to the e.g. site and select the option to log on automatically.",
	STATUS_VIRUS_INFECTED:                                              "The operation did not complete successfully because the file contains a virus.",
	STATUS_VIRUS_DELETED:                                               "This file contains a virus and cannot be opened. Due to the nature of this virus, the file has been removed from this location.",
	STATUS_BAD_MCFG_TABLE:                                              "The resources required for this device conflict with the MCFG table.",
	STATUS_CANNOT_BREAK_OPLOCK:                                         "The operation did not complete successfully because it would cause an oplock to be broken. The caller has requested that existing oplocks not be broken.",
	STATUS_WOW_ASSERTION:                                               "WOW Assertion Error.",
	STATUS_INVALID_SIGNATURE:                                           "The cryptographic signature is invalid.",
	STATUS_HMAC_NOT_SUPPORTED:                                          "The cryptographic provider does not support HMAC.",
	STATUS_IPSEC_QUEUE_OVERFLOW:                                        "The IPsec queue overflowed.",
	STATUS_ND_QUEUE_OVERFLOW:                                           "The neighbor discovery queue overflowed.",
	STATUS_HOPLIMIT_EXCEEDED:                                           "An Internet Control Message Protocol (ICMP) hop limit exceeded error was received.",
	STATUS_PROTOCOL_NOT_SUPPORTED:                                      "The protocol is not installed on the local machine.",
	STATUS_LOST_WRITEBEHIND_DATA_NETWORK_DISCONNECTED:                  "{Delayed Write Failed} Windows was unable to save all the data for the file %hs; the data has been lost. This error may be caused by network connectivity issues. Try to save this file elsewhere.",
	STATUS_LOST_WRITEBEHIND_DATA_NETWORK_SERVER_ERROR:                  "{Delayed Write Failed} Windows was unable to save all the data for the file %hs; the data has been lost. This error was returned by the server on which the file exists. Try to save this file elsewhere.",
	STATUS_LOST_WRITEBEHIND_DATA_LOCAL_DISK_ERROR:                      "{Delayed Write Failed} Windows was unable to save all the data for the file %hs; the data has been lost. This error may be caused if the device has been removed or the media is write-protected.",
	STATUS_XML_PARSE_ERROR:                                             "Windows was unable to parse the requested XML data.",
	STATUS_XMLDSIG_ERROR:                                               "An error was encountered while processing an XML digital signature.",
	STATUS_WRONG_COMPARTMENT:                                           "This indicates that the caller made the connection request in the wrong routing compartment.",
	STATUS_AUTHIP_FAILURE:                                              "This indicates that there was an AuthIP failure when attempting to connect to the remote host.",
	STATUS_DS_OID_MAPPED_GROUP_CANT_HAVE_MEMBERS:                       "OID mapped groups cannot have members.",
	STATUS_DS_OID_NOT_FOUND:                                            "The specified OID cannot be found.",
	STATUS_HASH_NOT_SUPPORTED:                                          "Hash generation for the specified version and hash type is not enabled on server.",
	STATUS_HASH_NOT_PRESENT:                                            "The hash requests is not present or not up to date with the current file contents.",
	STATUS_OFFLOAD_READ_FLT_NOT_SUPPORTED:                              "A file system filter on the server has not opted in for Offload Read support.",
	STATUS_OFFLOAD_WRITE_FLT_NOT_SUPPORTED:                             "A file system filter on the server has not opted in for Offload Write support.",
	STATUS_OFFLOAD_READ_FILE_NOT_SUPPORTED:                             "Offload read operations cannot be performed on:   Compressed files   Sparse files   Encrypted files   File system metadata files",
	STATUS_OFFLOAD_WRITE_FILE_NOT_SUPPORTED:                            "Offload write operations cannot be performed on:   Compressed files   Sparse files   Encrypted files   File system metadata files",
	DBG_NO_STATE_CHANGE:                                                "The debugger did not perform a state change.",
	DBG_APP_NOT_IDLE:                                                   "The debugger found that the application is not idle.",
	RPC_NT_INVALID_STRING_BINDING:                                      "The string binding is invalid.",
	RPC_NT_WRONG_KIND_OF_BINDING:                                       "The binding handle is not the correct type.",
	RPC_NT_INVALID_BINDING:                                             "The binding handle is invalid.",
	RPC_NT_PROTSEQ_NOT_SUPPORTED:                                       "The RPC protocol sequence is not supported.",
	RPC_NT_INVALID_RPC_PROTSEQ:                                         "The RPC protocol sequence is invalid.",
	RPC_NT_INVALID_STRING_UUID:                                         "The string UUID is invalid.",
	RPC_NT_INVALID_ENDPOINT_FORMAT:                                     "The endpoint format is invalid.",
	RPC_NT_INVALID_NET_ADDR:                                            "The network address is invalid.",
	RPC_NT_NO_ENDPOINT_FOUND:                                           "No endpoint was found.",
	RPC_NT_INVALID_TIMEOUT:                                             "The time-out value is invalid.",
	RPC_NT_OBJECT_NOT_FOUND:                                            "The object UUID was not found.",
	RPC_NT_ALREADY_REGISTERED:                                          "The object UUID has already been registered.",
	RPC_NT_TYPE_ALREADY_REGISTERED:                                     "The type UUID has already been registered.",
	RPC_NT_ALREADY_LISTENING:                                           "The RPC server is already listening.",
	RPC_NT_NO_PROTSEQS_REGISTERED:                                      "No protocol sequences have been registered.",
	RPC_NT_NOT_LISTENING:                                               "The RPC server is not listening.",
	RPC_NT_UNKNOWN_MGR_TYPE:                                            "The manager type is unknown.",
	RPC_NT_UNKNOWN_IF:                                                  "The interface is unknown.",
	RPC_NT_NO_BINDINGS:                                                 "There are no bindings.",
	RPC_NT_NO_PROTSEQS:                                                 "There are no protocol sequences.",
	RPC_NT_CANT_CREATE_ENDPOINT:                                        "The endpoint cannot be created.",
	RPC_NT_OUT_OF_RESOURCES:                                            "Insufficient resources are available to complete this operation.",
	RPC_NT_SERVER_UNAVAILABLE:                                          "The RPC server is unavailable.",
	RPC_NT_SERVER_TOO_BUSY:                                             "The RPC server is too busy to complete this operation.",
	RPC_NT_INVALID_NETWORK_OPTIONS:                                     "The network options are invalid.",
	RPC_NT_NO_CALL_ACTIVE:                                              "No RPCs are active on this thread.",
	RPC_NT_CALL_FAILED:                                                 "The RPC failed.",
	RPC_NT_CALL_FAILED_DNE:                                             "The RPC failed and did not execute.",
	RPC_NT_PROTOCOL_ERROR:                                              "An RPC protocol error occurred.",
	RPC_NT_UNSUPPORTED_TRANS_SYN:                                       "The RPC server does not support the transfer syntax.",
	RPC_NT_UNSUPPORTED_TYPE:                                            "The type UUID is not supported.",
	RPC_NT_INVALID_TAG:                                                 "The tag is invalid.",
	RPC_NT_INVALID_BOUND:                                               "The array bounds are invalid.",
	RPC_NT_NO_ENTRY_NAME:                                               "The binding does not contain an entry name.",
	RPC_NT_INVALID_NAME_SYNTAX:                                         "The name syntax is invalid.",
	RPC_NT_UNSUPPORTED_NAME_SYNTAX:                                     "The name syntax is not supported.",
	RPC_NT_UUID_NO_ADDRESS:                                             "No network address is available to construct a UUID.",
	RPC_NT_DUPLICATE_ENDPOINT:                                          "The endpoint is a duplicate.",
	RPC_NT_UNKNOWN_AUTHN_TYPE:                                          "The authentication type is unknown.",
	RPC_NT_MAX_CALLS_TOO_SMALL:                                         "The maximum number of calls is too small.",
	RPC_NT_STRING_TOO_LONG:                                             "The string is too long.",
	RPC_NT_PROTSEQ_NOT_FOUND:                                           "The RPC protocol sequence was not found.",
	RPC_NT_PROCNUM_OUT_OF_RANGE:                                        "The procedure number is out of range.",
	RPC_NT_BINDING_HAS_NO_AUTH:                                         "The binding does not contain any authentication information.",
	RPC_NT_UNKNOWN_AUTHN_SERVICE:                                       "The authentication service is unknown.",
	RPC_NT_UNKNOWN_AUTHN_LEVEL:                                         "The authentication level is unknown.",
	RPC_NT_INVALID_AUTH_IDENTITY:                                       "The security context is invalid.",
	RPC_NT_UNKNOWN_AUTHZ_SERVICE:                                       "The authorization service is unknown.",
	EPT_NT_INVALID_ENTRY:                                               "The entry is invalid.",
	EPT_NT_CANT_PERFORM_OP:                                             "The operation cannot be performed.",
	EPT_NT_NOT_REGISTERED:                                              "No more endpoints are available from the endpoint mapper.",
	RPC_NT_NOTHING_TO_EXPORT:                                           "No interfaces have been exported.",
	RPC_NT_INCOMPLETE_NAME:                                             "The entry name is incomplete.",
	RPC_NT_INVALID_VERS_OPTION:                                         "The version option is invalid.",
	RPC_NT_NO_MORE_MEMBERS:                                             "There are no more members.",
	RPC_NT_NOT_ALL_OBJS_UNEXPORTED:                                     "There is nothing to unexport.",
	RPC_NT_INTERFACE_NOT_FOUND:                                         "The interface was not found.",
	RPC_NT_ENTRY_ALREADY_EXISTS:                                        "The entry already exists.",
	RPC_NT_ENTRY_NOT_FOUND:                                             "The entry was not found.",
	RPC_NT_NAME_SERVICE_UNAVAILABLE:                                    "The name service is unavailable.",
	RPC_NT_INVALID_NAF_ID:                                              "The network address family is invalid.",
	RPC_NT_CANNOT_SUPPORT:                                              "The requested operation is not supported.",
	RPC_NT_NO_CONTEXT_AVAILABLE:                                        "No security context is available to allow impersonation.",
	RPC_NT_INTERNAL_ERROR:                                              "An internal error occurred in the RPC.",
	RPC_NT_ZERO_DIVIDE:                                                 "The RPC server attempted to divide an integer by zero.",
	RPC_NT_ADDRESS_ERROR:                                               "An addressing error occurred in the RPC server.",
	RPC_NT_FP_DIV_ZERO:                                                 "A floating point operation at the RPC server caused a divide by zero.",
	RPC_NT_FP_UNDERFLOW:                                                "A floating point underflow occurred at the RPC server.",
	RPC_NT_FP_OVERFLOW:                                                 "A floating point overflow occurred at the RPC server.",
	RPC_NT_CALL_IN_PROGRESS:                                            "An RPC is already in progress for this thread.",
	RPC_NT_NO_MORE_BINDINGS:                                            "There are no more bindings.",
	RPC_NT_GROUP_MEMBER_NOT_FOUND:                                      "The group member was not found.",
	EPT_NT_CANT_CREATE:                                                 "The endpoint mapper database entry could not be created.",
	RPC_NT_INVALID_OBJECT:                                              "The object UUID is the nil UUID.",
	RPC_NT_NO_INTERFACES:                                               "No interfaces have been registered.",
	RPC_NT_CALL_CANCELLED:                                              "The RPC was canceled.",
	RPC_NT_BINDING_INCOMPLETE:                                          "The binding handle does not contain all the required information.",
	RPC_NT_COMM_FAILURE:                                                "A communications failure occurred during an RPC.",
	RPC_NT_UNSUPPORTED_AUTHN_LEVEL:                                     "The requested authentication level is not supported.",
	RPC_NT_NO_PRINC_NAME:                                               "No principal name was registered.",
	RPC_NT_NOT_RPC_ERROR:                                               "The error specified is not a valid Windows RPC error code.",
	RPC_NT_SEC_PKG_ERROR:                                               "A security package-specific error occurred.",
	RPC_NT_NOT_CANCELLED:                                               "The thread was not canceled.",
	RPC_NT_INVALID_ASYNC_HANDLE:                                        "Invalid asynchronous RPC handle.",
	RPC_NT_INVALID_ASYNC_CALL:                                          "Invalid asynchronous RPC call handle for this operation.",
	RPC_NT_PROXY_ACCESS_DENIED:                                         "Access to the HTTP proxy is denied.",
	RPC_NT_NO_MORE_ENTRIES:                                             "The list of RPC servers available for auto-handle binding has been exhausted.",
	RPC_NT_SS_CHAR_TRANS_OPEN_FAIL:                                     "The file designated by DCERPCCHARTRANS cannot be opened.",
	RPC_NT_SS_CHAR_TRANS_SHORT_FILE:                                    "The file containing the character translation table has fewer than 512 bytes.",
	RPC_NT_SS_IN_NULL_CONTEXT:                                          "A null context handle is passed as an [in] parameter.",
	RPC_NT_SS_CONTEXT_MISMATCH:                                         "The context handle does not match any known context handles.",
	RPC_NT_SS_CONTEXT_DAMAGED:                                          "The context handle changed during a call.",
	RPC_NT_SS_HANDLES_MISMATCH:                                         "The binding handles passed to an RPC do not match.",
	RPC_NT_SS_CANNOT_GET_CALL_HANDLE:                                   "The stub is unable to get the call handle.",
	RPC_NT_NULL_REF_POINTER:                                            "A null reference pointer was passed to the stub.",
	RPC_NT_ENUM_VALUE_OUT_OF_RANGE:                                     "The enumeration value is out of range.",
	RPC_NT_BYTE_COUNT_TOO_SMALL:                                        "The byte count is too small.",
	RPC_NT_BAD_STUB_DATA:                                               "The stub received bad data.",
	RPC_NT_INVALID_ES_ACTION:                                           "Invalid operation on the encoding/decoding handle.",
	RPC_NT_WRONG_ES_VERSION:                                            "Incompatible version of the serializing package.",
	RPC_NT_WRONG_STUB_VERSION:                                          "Incompatible version of the RPC stub.",
	RPC_NT_INVALID_PIPE_OBJECT:                                         "The RPC pipe object is invalid or corrupt.",
	RPC_NT_INVALID_PIPE_OPERATION:                                      "An invalid operation was attempted on an RPC pipe object.",
	RPC_NT_WRONG_PIPE_VERSION:                                          "Unsupported RPC pipe version.",
	RPC_NT_PIPE_CLOSED:                                                 "The RPC pipe object has already been closed.",
	RPC_NT_PIPE_DISCIPLINE_ERROR:                                       "The RPC call completed before all pipes were processed.",
	RPC_NT_PIPE_EMPTY:                                                  "No more data is available from the RPC pipe.",
	STATUS_PNP_BAD_MPS_TABLE:                                           "A device is missing in the system BIOS MPS table. This device will not be used. Contact your system vendor for a system BIOS update.",
	STATUS_PNP_TRANSLATION_FAILED:                                      "A translator failed to translate resources.",
	STATUS_PNP_IRQ_TRANSLATION_FAILED:                                  "An IRQ translator failed to translate resources.",
	STATUS_PNP_INVALID_ID:                                              "Driver %2 returned an invalid ID for a child device (%3).",
	STATUS_IO_REISSUE_AS_CACHED:                                        "Reissue the given operation as a cached I/O operation",
	STATUS_CTX_WINSTATION_NAME_INVALID:                                 "Session name %1 is invalid.",
	STATUS_CTX_INVALID_PD:                                              "The protocol driver %1 is invalid.",
	STATUS_CTX_PD_NOT_FOUND:                                            "The protocol driver %1 was not found in the system path.",
	STATUS_CTX_CLOSE_PENDING:                                           "A close operation is pending on the terminal connection.",
	STATUS_CTX_NO_OUTBUF:                                               "No free output buffers are available.",
	STATUS_CTX_MODEM_INF_NOT_FOUND:                                     "The MODEM.INF file was not found.",
	STATUS_CTX_INVALID_MODEMNAME:                                       "The modem (%1) was not found in the MODEM.INF file.",
	STATUS_CTX_RESPONSE_ERROR:                                          "The modem did not accept the command sent to it. Verify that the configured modem name matches the attached modem.",
	STATUS_CTX_MODEM_RESPONSE_TIMEOUT:                                  "The modem did not respond to the command sent to it. Verify that the modem cable is properly attached and the modem is turned on.",
	STATUS_CTX_MODEM_RESPONSE_NO_CARRIER:                               "Carrier detection has failed or the carrier has been dropped due to disconnection.",
	STATUS_CTX_MODEM_RESPONSE_NO_DIALTONE:                              "A dial tone was not detected within the required time. Verify that the phone cable is properly attached and functional.",
	STATUS_CTX_MODEM_RESPONSE_BUSY:                                     "A busy signal was detected at a remote site on callback.",
	STATUS_CTX_MODEM_RESPONSE_VOICE:                                    "A voice was detected at a remote site on callback.",
	STATUS_CTX_TD_ERROR:                                                "Transport driver error.",
	STATUS_CTX_LICENSE_CLIENT_INVALID:                                  "The client you are using is not licensed to use this system. Your logon request is denied.",
	STATUS_CTX_LICENSE_NOT_AVAILABLE:                                   "The system has reached its licensed logon limit. Try again later.",
	STATUS_CTX_LICENSE_EXPIRED:                                         "The system license has expired. Your logon request is denied.",
	STATUS_CTX_WINSTATION_NOT_FOUND:                                    "The specified session cannot be found.",
	STATUS_CTX_WINSTATION_NAME_COLLISION:                               "The specified session name is already in use.",
	STATUS_CTX_WINSTATION_BUSY:                                         "The requested operation cannot be completed because the terminal connection is currently processing a connect, disconnect, reset, or delete operation.",
	STATUS_CTX_BAD_VIDEO_MODE:                                          "An attempt has been made to connect to a session whose video mode is not supported by the current client.",
	STATUS_CTX_GRAPHICS_INVALID:                                        "The application attempted to enable DOS graphics mode. DOS graphics mode is not supported.",
	STATUS_CTX_NOT_CONSOLE:                                             "The requested operation can be performed only on the system console. This is most often the result of a driver or system DLL requiring direct console access.",
	STATUS_CTX_CLIENT_QUERY_TIMEOUT:                                    "The client failed to respond to the server connect message.",
	STATUS_CTX_CONSOLE_DISCONNECT:                                      "Disconnecting the console session is not supported.",
	STATUS_CTX_CONSOLE_CONNECT:                                         "Reconnecting a disconnected session to the console is not supported.",
	STATUS_CTX_SHADOW_DENIED:                                           "The request to control another session remotely was denied.",
	STATUS_CTX_WINSTATION_ACCESS_DENIED:                                "A process has requested access to a session, but has not been granted those access rights.",
	STATUS_CTX_INVALID_WD:                                              "The terminal connection driver %1 is invalid.",
	STATUS_CTX_WD_NOT_FOUND:                                            "The terminal connection driver %1 was not found in the system path.",
	STATUS_CTX_SHADOW_INVALID:                                          "The requested session cannot be controlled remotely. You cannot control your own session, a session that is trying to control your session, a session that has no user logged on, or other sessions from the console.",
	STATUS_CTX_SHADOW_DISABLED:                                         "The requested session is not configured to allow remote control.",
	STATUS_RDP_PROTOCOL_ERROR:                                          "The RDP protocol component %2 detected an error in the protocol stream and has disconnected the client.",
	STATUS_CTX_CLIENT_LICENSE_NOT_SET:                                  "Your request to connect to this terminal server has been rejected. Your terminal server client license number has not been entered for this copy of the terminal client. Contact your system administrator for help in entering a valid, unique license number for this terminal server client. Click OK to continue.",
	STATUS_CTX_CLIENT_LICENSE_IN_USE:                                   "Your request to connect to this terminal server has been rejected. Your terminal server client license number is currently being used by another user. Contact your system administrator to obtain a new copy of the terminal server client with a valid, unique license number. Click OK to continue.",
	STATUS_CTX_SHADOW_ENDED_BY_MODE_CHANGE:                             "The remote control of the console was terminated because the display mode was changed. Changing the display mode in a remote control session is not supported.",
	STATUS_CTX_SHADOW_NOT_RUNNING:                                      "Remote control could not be terminated because the specified session is not currently being remotely controlled.",
	STATUS_CTX_LOGON_DISABLED:                                          "Your interactive logon privilege has been disabled. Contact your system administrator.",
	STATUS_CTX_SECURITY_LAYER_ERROR:                                    "The terminal server security layer detected an error in the protocol stream and has disconnected the client.",
	STATUS_TS_INCOMPATIBLE_SESSIONS:                                    "The target session is incompatible with the current session.",
	STATUS_MUI_FILE_NOT_FOUND:                                          "The resource loader failed to find an MUI file.",
	STATUS_MUI_INVALID_FILE:                                            "The resource loader failed to load an MUI file because the file failed to pass validation.",
	STATUS_MUI_INVALID_RC_CONFIG:                                       "The RC manifest is corrupted with garbage data, is an unsupported version, or is missing a required item.",
	STATUS_MUI_INVALID_LOCALE_NAME:                                     "The RC manifest has an invalid culture name.",
	STATUS_MUI_INVALID_ULTIMATEFALLBACK_NAME:                           "The RC manifest has and invalid ultimate fallback name.",
	STATUS_MUI_FILE_NOT_LOADED:                                         "The resource loader cache does not have a loaded MUI entry.",
	STATUS_RESOURCE_ENUM_USER_STOP:                                     "The user stopped resource enumeration.",
	STATUS_CLUSTER_INVALID_NODE:                                        "The cluster node is not valid.",
	STATUS_CLUSTER_NODE_EXISTS:                                         "The cluster node already exists.",
	STATUS_CLUSTER_JOIN_IN_PROGRESS:                                    "A node is in the process of joining the cluster.",
	STATUS_CLUSTER_NODE_NOT_FOUND:                                      "The cluster node was not found.",
	STATUS_CLUSTER_LOCAL_NODE_NOT_FOUND:                                "The cluster local node information was not found.",
	STATUS_CLUSTER_NETWORK_EXISTS:                                      "The cluster network already exists.",
	STATUS_CLUSTER_NETWORK_NOT_FOUND:                                   "The cluster network was not found.",
	STATUS_CLUSTER_NETINTERFACE_EXISTS:                                 "The cluster network interface already exists.",
	STATUS_CLUSTER_NETINTERFACE_NOT_FOUND:                              "The cluster network interface was not found.",
	STATUS_CLUSTER_INVALID_REQUEST:                                     "The cluster request is not valid for this object.",
	STATUS_CLUSTER_INVALID_NETWORK_PROVIDER:                            "The cluster network provider is not valid.",
	STATUS_CLUSTER_NODE_DOWN:                                           "The cluster node is down.",
	STATUS_CLUSTER_NODE_UNREACHABLE:                                    "The cluster node is not reachable.",
	STATUS_CLUSTER_NODE_NOT_MEMBER:                                     "The cluster node is not a member of the cluster.",
	STATUS_CLUSTER_JOIN_NOT_IN_PROGRESS:                                "A cluster join operation is not in progress.",
	STATUS_CLUSTER_INVALID_NETWORK:                                     "The cluster network is not valid.",
	STATUS_CLUSTER_NO_NET_ADAPTERS:                                     "No network adapters are available.",
	STATUS_CLUSTER_NODE_UP:                                             "The cluster node is up.",
	STATUS_CLUSTER_NODE_PAUSED:                                         "The cluster node is paused.",
	STATUS_CLUSTER_NODE_NOT_PAUSED:                                     "The cluster node is not paused.",
	STATUS_CLUSTER_NO_SECURITY_CONTEXT:                                 "No cluster security context is available.",
	STATUS_CLUSTER_NETWORK_NOT_INTERNAL:                                "The cluster network is not configured for internal cluster communication.",
	STATUS_CLUSTER_POISONED:                                            "The cluster node has been poisoned.",
	STATUS_ACPI_INVALID_OPCODE:                                         "An attempt was made to run an invalid AML opcode.",
	STATUS_ACPI_STACK_OVERFLOW:                                         "The AML interpreter stack has overflowed.",
	STATUS_ACPI_ASSERT_FAILED:                                          "An inconsistent state has occurred.",
	STATUS_ACPI_INVALID_INDEX:                                          "An attempt was made to access an array outside its bounds.",
	STATUS_ACPI_INVALID_ARGUMENT:                                       "A required argument was not specified.",
	STATUS_ACPI_FATAL:                                                  "A fatal error has occurred.",
	STATUS_ACPI_INVALID_SUPERNAME:                                      "An invalid SuperName was specified.",
	STATUS_ACPI_INVALID_ARGTYPE:                                        "An argument with an incorrect type was specified.",
	STATUS_ACPI_INVALID_OBJTYPE:                                        "An object with an incorrect type was specified.",
	STATUS_ACPI_INVALID_TARGETTYPE:                                     "A target with an incorrect type was specified.",
	STATUS_ACPI_INCORRECT_ARGUMENT_COUNT:                               "An incorrect number of arguments was specified.",
	STATUS_ACPI_ADDRESS_NOT_MAPPED:                                     "An address failed to translate.",
	STATUS_ACPI_INVALID_EVENTTYPE:                                      "An incorrect event type was specified.",
	STATUS_ACPI_HANDLER_COLLISION:                                      "A handler for the target already exists.",
	STATUS_ACPI_INVALID_DATA:                                           "Invalid data for the target was specified.",
	STATUS_ACPI_INVALID_REGION:                                         "An invalid region for the target was specified.",
	STATUS_ACPI_INVALID_ACCESS_SIZE:                                    "An attempt was made to access a field outside the defined range.",
	STATUS_ACPI_ACQUIRE_GLOBAL_LOCK:                                    "The global system lock could not be acquired.",
	STATUS_ACPI_ALREADY_INITIALIZED:                                    "An attempt was made to reinitialize the ACPI subsystem.",
	STATUS_ACPI_NOT_INITIALIZED:                                        "The ACPI subsystem has not been initialized.",
	STATUS_ACPI_INVALID_MUTEX_LEVEL:                                    "An incorrect mutex was specified.",
	STATUS_ACPI_MUTEX_NOT_OWNED:                                        "The mutex is not currently owned.",
	STATUS_ACPI_MUTEX_NOT_OWNER:                                        "An attempt was made to access the mutex by a process that was not the owner.",
	STATUS_ACPI_RS_ACCESS:                                              "An error occurred during an access to region space.",
	STATUS_ACPI_INVALID_TABLE:                                          "An attempt was made to use an incorrect table.",
	STATUS_ACPI_REG_HANDLER_FAILED:                                     "The registration of an ACPI event failed.",
	STATUS_ACPI_POWER_REQUEST_FAILED:                                   "An ACPI power object failed to transition state.",
	STATUS_SXS_SECTION_NOT_FOUND:                                       "The requested section is not present in the activation context.",
	STATUS_SXS_CANT_GEN_ACTCTX:                                         "Windows was unble to process the application binding information. Refer to the system event log for further information.",
	STATUS_SXS_INVALID_ACTCTXDATA_FORMAT:                               "The application binding data format is invalid.",
	STATUS_SXS_ASSEMBLY_NOT_FOUND:                                      "The referenced assembly is not installed on the system.",
	STATUS_SXS_MANIFEST_FORMAT_ERROR:                                   "The manifest file does not begin with the required tag and format information.",
	STATUS_SXS_MANIFEST_PARSE_ERROR:                                    "The manifest file contains one or more syntax errors.",
	STATUS_SXS_ACTIVATION_CONTEXT_DISABLED:                             "The application attempted to activate a disabled activation context.",
	STATUS_SXS_KEY_NOT_FOUND:                                           "The requested lookup key was not found in any active activation context.",
	STATUS_SXS_VERSION_CONFLICT:                                        "A component version required by the application conflicts with another component version that is already active.",
	STATUS_SXS_WRONG_SECTION_TYPE:                                      "The type requested activation context section does not match the query API used.",
	STATUS_SXS_THREAD_QUERIES_DISABLED:                                 "Lack of system resources has required isolated activation to be disabled for the current thread of execution.",
	STATUS_SXS_ASSEMBLY_MISSING:                                        "The referenced assembly could not be found.",
	STATUS_SXS_PROCESS_DEFAULT_ALREADY_SET:                             "An attempt to set the process default activation context failed because the process default activation context was already set.",
	STATUS_SXS_EARLY_DEACTIVATION:                                      "The activation context being deactivated is not the most recently activated one.",
	STATUS_SXS_INVALID_DEACTIVATION:                                    "The activation context being deactivated is not active for the current thread of execution.",
	STATUS_SXS_MULTIPLE_DEACTIVATION:                                   "The activation context being deactivated has already been deactivated.",
	STATUS_SXS_SYSTEM_DEFAULT_ACTIVATION_CONTEXT_EMPTY:                 "The activation context of the system default assembly could not be generated.",
	STATUS_SXS_PROCESS_TERMINATION_REQUESTED:                           "A component used by the isolation facility has requested that the process be terminated.",
	STATUS_SXS_CORRUPT_ACTIVATION_STACK:                                "The activation context activation stack for the running thread of execution is corrupt.",
	STATUS_SXS_CORRUPTION:                                              "The application isolation metadata for this process or thread has become corrupt.",
	STATUS_SXS_INVALID_IDENTITY_ATTRIBUTE_VALUE:                        "The value of an attribute in an identity is not within the legal range.",
	STATUS_SXS_INVALID_IDENTITY_ATTRIBUTE_NAME:                         "The name of an attribute in an identity is not within the legal range.",
	STATUS_SXS_IDENTITY_DUPLICATE_ATTRIBUTE:                            "An identity contains two definitions for the same attribute.",
	STATUS_SXS_IDENTITY_PARSE_ERROR:                                    "The identity string is malformed. This may be due to a trailing comma, more than two unnamed attributes, a missing attribute name, or a missing attribute value.",
	STATUS_SXS_COMPONENT_STORE_CORRUPT:                                 "The component store has become corrupted.",
	STATUS_SXS_FILE_HASH_MISMATCH:                                      "A component's file does not match the verification information present in the component manifest.",
	STATUS_SXS_MANIFEST_IDENTITY_SAME_BUT_CONTENTS_DIFFERENT:           "The identities of the manifests are identical, but their contents are different.",
	STATUS_SXS_IDENTITIES_DIFFERENT:                                    "The component identities are different.",
	STATUS_SXS_ASSEMBLY_IS_NOT_A_DEPLOYMENT:                            "The assembly is not a deployment.",
	STATUS_SXS_FILE_NOT_PART_OF_ASSEMBLY:                               "The file is not a part of the assembly.",
	STATUS_ADVANCED_INSTALLER_FAILED:                                   "An advanced installer failed during setup or servicing.",
	STATUS_XML_ENCODING_MISMATCH:                                       "The character encoding in the XML declaration did not match the encoding used in the document.",
	STATUS_SXS_MANIFEST_TOO_BIG:                                        "The size of the manifest exceeds the maximum allowed.",
	STATUS_SXS_SETTING_NOT_REGISTERED:                                  "The setting is not registered.",
	STATUS_SXS_TRANSACTION_CLOSURE_INCOMPLETE:                          "One or more required transaction members are not present.",
	STATUS_SMI_PRIMITIVE_INSTALLER_FAILED:                              "The SMI primitive installer failed during setup or servicing.",
	STATUS_GENERIC_COMMAND_FAILED:                                      "A generic command executable returned a result that indicates failure.",
	STATUS_SXS_FILE_HASH_MISSING:                                       "A component is missing file verification information in its manifest.",
	STATUS_TRANSACTIONAL_CONFLICT:                                      "The function attempted to use a name that is reserved for use by another transaction.",
	STATUS_INVALID_TRANSACTION:                                         "The transaction handle associated with this operation is invalid.",
	STATUS_TRANSACTION_NOT_ACTIVE:                                      "The requested operation was made in the context of a transaction that is no longer active.",
	STATUS_TM_INITIALIZATION_FAILED:                                    "The transaction manager was unable to be successfully initialized. Transacted operations are not supported.",
	STATUS_RM_NOT_ACTIVE:                                               "Transaction support within the specified file system resource manager was not started or was shut down due to an error.",
	STATUS_RM_METADATA_CORRUPT:                                         "The metadata of the resource manager has been corrupted. The resource manager will not function.",
	STATUS_TRANSACTION_NOT_JOINED:                                      "The resource manager attempted to prepare a transaction that it has not successfully joined.",
	STATUS_DIRECTORY_NOT_RM:                                            "The specified directory does not contain a file system resource manager.",
	STATUS_TRANSACTIONS_UNSUPPORTED_REMOTE:                             "The remote server or share does not support transacted file operations.",
	STATUS_LOG_RESIZE_INVALID_SIZE:                                     "The requested log size for the file system resource manager is invalid.",
	STATUS_REMOTE_FILE_VERSION_MISMATCH:                                "The remote server sent mismatching version number or Fid for a file opened with transactions.",
	STATUS_CRM_PROTOCOL_ALREADY_EXISTS:                                 "The resource manager tried to register a protocol that already exists.",
	STATUS_TRANSACTION_PROPAGATION_FAILED:                              "The attempt to propagate the transaction failed.",
	STATUS_CRM_PROTOCOL_NOT_FOUND:                                      "The requested propagation protocol was not registered as a CRM.",
	STATUS_TRANSACTION_SUPERIOR_EXISTS:                                 "The transaction object already has a superior enlistment, and the caller attempted an operation that would have created a new superior. Only a single superior enlistment is allowed.",
	STATUS_TRANSACTION_REQUEST_NOT_VALID:                               "The requested operation is not valid on the transaction object in its current state.",
	STATUS_TRANSACTION_NOT_REQUESTED:                                   "The caller has called a response API, but the response is not expected because the transaction manager did not issue the corresponding request to the caller.",
	STATUS_TRANSACTION_ALREADY_ABORTED:                                 "It is too late to perform the requested operation, because the transaction has already been aborted.",
	STATUS_TRANSACTION_ALREADY_COMMITTED:                               "It is too late to perform the requested operation, because the transaction has already been committed.",
	STATUS_TRANSACTION_INVALID_MARSHALL_BUFFER:                         "The buffer passed in to NtPushTransaction or NtPullTransaction is not in a valid format.",
	STATUS_CURRENT_TRANSACTION_NOT_VALID:                               "The current transaction context associated with the thread is not a valid handle to a transaction object.",
	STATUS_LOG_GROWTH_FAILED:                                           "An attempt to create space in the transactional resource manager's log failed. The failure status has been recorded in the event log.",
	STATUS_OBJECT_NO_LONGER_EXISTS:                                     "The object (file, stream, or link) that corresponds to the handle has been deleted by a transaction savepoint rollback.",
	STATUS_STREAM_MINIVERSION_NOT_FOUND:                                "The specified file miniversion was not found for this transacted file open.",
	STATUS_STREAM_MINIVERSION_NOT_VALID:                                "The specified file miniversion was found but has been invalidated. The most likely cause is a transaction savepoint rollback.",
	STATUS_MINIVERSION_INACCESSIBLE_FROM_SPECIFIED_TRANSACTION:         "A miniversion may be opened only in the context of the transaction that created it.",
	STATUS_CANT_OPEN_MINIVERSION_WITH_MODIFY_INTENT:                    "It is not possible to open a miniversion with modify access.",
	STATUS_CANT_CREATE_MORE_STREAM_MINIVERSIONS:                        "It is not possible to create any more miniversions for this stream.",
	STATUS_HANDLE_NO_LONGER_VALID:                                      "The handle has been invalidated by a transaction. The most likely cause is the presence of memory mapping on a file or an open handle when the transaction ended or rolled back to savepoint.",
	STATUS_LOG_CORRUPTION_DETECTED:                                     "The log data is corrupt.",
	STATUS_RM_DISCONNECTED:                                             "The transaction outcome is unavailable because the resource manager responsible for it is disconnected.",
	STATUS_ENLISTMENT_NOT_SUPERIOR:                                     "The request was rejected because the enlistment in question is not a superior enlistment.",
	STATUS_FILE_IDENTITY_NOT_PERSISTENT:                                "The file cannot be opened in a transaction because its identity depends on the outcome of an unresolved transaction.",
	STATUS_CANT_BREAK_TRANSACTIONAL_DEPENDENCY:                         "The operation cannot be performed because another transaction is depending on this property not changing.",
	STATUS_CANT_CROSS_RM_BOUNDARY:                                      "The operation would involve a single file with two transactional resource managers and is, therefore, not allowed.",
	STATUS_TXF_DIR_NOT_EMPTY:                                           "The $Txf directory must be empty for this operation to succeed.",
	STATUS_INDOUBT_TRANSACTIONS_EXIST:                                  "The operation would leave a transactional resource manager in an inconsistent state and is therefore not allowed.",
	STATUS_TM_VOLATILE:                                                 "The operation could not be completed because the transaction manager does not have a log.",
	STATUS_ROLLBACK_TIMER_EXPIRED:                                      "A rollback could not be scheduled because a previously scheduled rollback has already executed or been queued for execution.",
	STATUS_TXF_ATTRIBUTE_CORRUPT:                                       "The transactional metadata attribute on the file or directory %hs is corrupt and unreadable.",
	STATUS_EFS_NOT_ALLOWED_IN_TRANSACTION:                              "The encryption operation could not be completed because a transaction is active.",
	STATUS_TRANSACTIONAL_OPEN_NOT_ALLOWED:                              "This object is not allowed to be opened in a transaction.",
	STATUS_TRANSACTED_MAPPING_UNSUPPORTED_REMOTE:                       "Memory mapping (creating a mapped section) a remote file under a transaction is not supported.",
	STATUS_TRANSACTION_REQUIRED_PROMOTION:                              "Promotion was required to allow the resource manager to enlist, but the transaction was set to disallow it.",
	STATUS_CANNOT_EXECUTE_FILE_IN_TRANSACTION:                          "This file is open for modification in an unresolved transaction and may be opened for execute only by a transacted reader.",
	STATUS_TRANSACTIONS_NOT_FROZEN:                                     "The request to thaw frozen transactions was ignored because transactions were not previously frozen.",
	STATUS_TRANSACTION_FREEZE_IN_PROGRESS:                              "Transactions cannot be frozen because a freeze is already in progress.",
	STATUS_NOT_SNAPSHOT_VOLUME:                                         "The target volume is not a snapshot volume. This operation is valid only on a volume mounted as a snapshot.",
	STATUS_NO_SAVEPOINT_WITH_OPEN_FILES:                                "The savepoint operation failed because files are open on the transaction, which is not permitted.",
	STATUS_SPARSE_NOT_ALLOWED_IN_TRANSACTION:                           "The sparse operation could not be completed because a transaction is active on the file.",
	STATUS_TM_IDENTITY_MISMATCH:                                        "The call to create a transaction manager object failed because the Tm Identity that is stored in the log file does not match the Tm Identity that was passed in as an argument.",
	STATUS_FLOATED_SECTION:                                             "I/O was attempted on a section object that has been floated as a result of a transaction ending. There is no valid data.",
	STATUS_CANNOT_ACCEPT_TRANSACTED_WORK:                               "The transactional resource manager cannot currently accept transacted work due to a transient condition, such as low resources.",
	STATUS_CANNOT_ABORT_TRANSACTIONS:                                   "The transactional resource manager had too many transactions outstanding that could not be aborted. The transactional resource manager has been shut down.",
	STATUS_TRANSACTION_NOT_FOUND:                                       "The specified transaction was unable to be opened because it was not found.",
	STATUS_RESOURCEMANAGER_NOT_FOUND:                                   "The specified resource manager was unable to be opened because it was not found.",
	STATUS_ENLISTMENT_NOT_FOUND:                                        "The specified enlistment was unable to be opened because it was not found.",
	STATUS_TRANSACTIONMANAGER_NOT_FOUND:                                "The specified transaction manager was unable to be opened because it was not found.",
	STATUS_TRANSACTIONMANAGER_NOT_ONLINE:                               "The specified resource manager was unable to create an enlistment because its associated transaction manager is not online.",
	STATUS_TRANSACTIONMANAGER_RECOVERY_NAME_COLLISION:                  "The specified transaction manager was unable to create the objects contained in its log file in the Ob namespace. Therefore, the transaction manager was unable to recover.",
	STATUS_TRANSACTION_NOT_ROOT:                                        "The call to create a superior enlistment on this transaction object could not be completed because the transaction object specified for the enlistment is a subordinate branch of the transaction. Only the root of the transaction can be enlisted as a superior.",
	STATUS_TRANSACTION_OBJECT_EXPIRED:                                  "Because the associated transaction manager or resource manager has been closed, the handle is no longer valid.",
	STATUS_COMPRESSION_NOT_ALLOWED_IN_TRANSACTION:                      "The compression operation could not be completed because a transaction is active on the file.",
	STATUS_TRANSACTION_RESPONSE_NOT_ENLISTED:                           "The specified operation could not be performed on this superior enlistment because the enlistment was not created with the corresponding completion response in the NotificationMask.",
	STATUS_TRANSACTION_RECORD_TOO_LONG:                                 "The specified operation could not be performed because the record to be logged was too long. This can occur because either there are too many enlistments on this transaction or the combined RecoveryInformation being logged on behalf of those enlistments is too long.",
	STATUS_NO_LINK_TRACKING_IN_TRANSACTION:                             "The link-tracking operation could not be completed because a transaction is active.",
	STATUS_OPERATION_NOT_SUPPORTED_IN_TRANSACTION:                      "This operation cannot be performed in a transaction.",
	STATUS_TRANSACTION_INTEGRITY_VIOLATED:                              "The kernel transaction manager had to abort or forget the transaction because it blocked forward progress.",
	STATUS_EXPIRED_HANDLE:                                              "The handle is no longer properly associated with its transaction.\u00a0 It may have been opened in a transactional resource manager that was subsequently forced to restart.\u00a0 Please close the handle and open a new one.",
	STATUS_TRANSACTION_NOT_ENLISTED:                                    "The specified operation could not be performed because the resource manager is not enlisted in the transaction.",
	STATUS_LOG_SECTOR_INVALID:                                          "The log service found an invalid log sector.",
	STATUS_LOG_SECTOR_PARITY_INVALID:                                   "The log service encountered a log sector with invalid block parity.",
	STATUS_LOG_SECTOR_REMAPPED:                                         "The log service encountered a remapped log sector.",
	STATUS_LOG_BLOCK_INCOMPLETE:                                        "The log service encountered a partial or incomplete log block.",
	STATUS_LOG_INVALID_RANGE:                                           "The log service encountered an attempt to access data outside the active log range.",
	STATUS_LOG_BLOCKS_EXHAUSTED:                                        "The log service user-log marshaling buffers are exhausted.",
	STATUS_LOG_READ_CONTEXT_INVALID:                                    "The log service encountered an attempt to read from a marshaling area with an invalid read context.",
	STATUS_LOG_RESTART_INVALID:                                         "The log service encountered an invalid log restart area.",
	STATUS_LOG_BLOCK_VERSION:                                           "The log service encountered an invalid log block version.",
	STATUS_LOG_BLOCK_INVALID:                                           "The log service encountered an invalid log block.",
	STATUS_LOG_READ_MODE_INVALID:                                       "The log service encountered an attempt to read the log with an invalid read mode.",
	STATUS_LOG_METADATA_CORRUPT:                                        "The log service encountered a corrupted metadata file.",
	STATUS_LOG_METADATA_INVALID:                                        "The log service encountered a metadata file that could not be created by the log file system.",
	STATUS_LOG_METADATA_INCONSISTENT:                                   "The log service encountered a metadata file with inconsistent data.",
	STATUS_LOG_RESERVATION_INVALID:                                     "The log service encountered an attempt to erroneously allocate or dispose reservation space.",
	STATUS_LOG_CANT_DELETE:                                             "The log service cannot delete the log file or the file system container.",
	STATUS_LOG_CONTAINER_LIMIT_EXCEEDED:                                "The log service has reached the maximum allowable containers allocated to a log file.",
	STATUS_LOG_START_OF_LOG:                                            "The log service has attempted to read or write backward past the start of the log.",
	STATUS_LOG_POLICY_ALREADY_INSTALLED:                                "The log policy could not be installed because a policy of the same type is already present.",
	STATUS_LOG_POLICY_NOT_INSTALLED:                                    "The log policy in question was not installed at the time of the request.",
	STATUS_LOG_POLICY_INVALID:                                          "The installed set of policies on the log is invalid.",
	STATUS_LOG_POLICY_CONFLICT:                                         "A policy on the log in question prevented the operation from completing.",
	STATUS_LOG_PINNED_ARCHIVE_TAIL:                                     "The log space cannot be reclaimed because the log is pinned by the archive tail.",
	STATUS_LOG_RECORD_NONEXISTENT:                                      "The log record is not a record in the log file.",
	STATUS_LOG_RECORDS_RESERVED_INVALID:                                "The number of reserved log records or the adjustment of the number of reserved log records is invalid.",
	STATUS_LOG_SPACE_RESERVED_INVALID:                                  "The reserved log space or the adjustment of the log space is invalid.",
	STATUS_LOG_TAIL_INVALID:                                            "A new or existing archive tail or the base of the active log is invalid.",
	STATUS_LOG_FULL:                                                    "The log space is exhausted.",
	STATUS_LOG_MULTIPLEXED:                                             "The log is multiplexed; no direct writes to the physical log are allowed.",
	STATUS_LOG_DEDICATED:                                               "The operation failed because the log is dedicated.",
	STATUS_LOG_ARCHIVE_NOT_IN_PROGRESS:                                 "The operation requires an archive context.",
	STATUS_LOG_ARCHIVE_IN_PROGRESS:                                     "Log archival is in progress.",
	STATUS_LOG_EPHEMERAL:                                               "The operation requires a nonephemeral log, but the log is ephemeral.",
	STATUS_LOG_NOT_ENOUGH_CONTAINERS:                                   "The log must have at least two containers before it can be read from or written to.",
	STATUS_LOG_CLIENT_ALREADY_REGISTERED:                               "A log client has already registered on the stream.",
	STATUS_LOG_CLIENT_NOT_REGISTERED:                                   "A log client has not been registered on the stream.",
	STATUS_LOG_FULL_HANDLER_IN_PROGRESS:                                "A request has already been made to handle the log full condition.",
	STATUS_LOG_CONTAINER_READ_FAILED:                                   "The log service encountered an error when attempting to read from a log container.",
	STATUS_LOG_CONTAINER_WRITE_FAILED:                                  "The log service encountered an error when attempting to write to a log container.",
	STATUS_LOG_CONTAINER_OPEN_FAILED:                                   "The log service encountered an error when attempting to open a log container.",
	STATUS_LOG_CONTAINER_STATE_INVALID:                                 "The log service encountered an invalid container state when attempting a requested action.",
	STATUS_LOG_STATE_INVALID:                                           "The log service is not in the correct state to perform a requested action.",
	STATUS_LOG_PINNED:                                                  "The log space cannot be reclaimed because the log is pinned.",
	STATUS_LOG_METADATA_FLUSH_FAILED:                                   "The log metadata flush failed.",
	STATUS_LOG_INCONSISTENT_SECURITY:                                   "Security on the log and its containers is inconsistent.",
	STATUS_LOG_APPENDED_FLUSH_FAILED:                                   "Records were appended to the log or reservation changes were made, but the log could not be flushed.",
	STATUS_LOG_PINNED_RESERVATION:                                      "The log is pinned due to reservation consuming most of the log space. Free some reserved records to make space available.",
	STATUS_VIDEO_HUNG_DISPLAY_DRIVER_THREAD:                            "{Display Driver Stopped Responding} The %hs display driver has stopped working normally. Save your work and reboot the system to restore full display functionality. The next time you reboot the computer, a dialog box will allow you to upload data about this failure to Microsoft.",
	STATUS_FLT_NO_HANDLER_DEFINED:                                      "A handler was not defined by the filter for this operation.",
	STATUS_FLT_CONTEXT_ALREADY_DEFINED:                                 "A context is already defined for this object.",
	STATUS_FLT_INVALID_ASYNCHRONOUS_REQUEST:                            "Asynchronous requests are not valid for this operation.",
	STATUS_FLT_DISALLOW_FAST_IO:                                        "This is an internal error code used by the filter manager to determine if a fast I/O operation should be forced down the input/output request packet (IRP) path. Minifilters should never return this value.",
	STATUS_FLT_INVALID_NAME_REQUEST:                                    "An invalid name request was made. The name requested cannot be retrieved at this time.",
	STATUS_FLT_NOT_SAFE_TO_POST_OPERATION:                              "Posting this operation to a worker thread for further processing is not safe at this time because it could lead to a system deadlock.",
	STATUS_FLT_NOT_INITIALIZED:                                         "The Filter Manager was not initialized when a filter tried to register. Make sure that the Filter Manager is loaded as a driver.",
	STATUS_FLT_FILTER_NOT_READY:                                        "The filter is not ready for attachment to volumes because it has not finished initializing (FltStartFiltering has not been called).",
	STATUS_FLT_POST_OPERATION_CLEANUP:                                  "The filter must clean up any operation-specific context at this time because it is being removed from the system before the operation is completed by the lower drivers.",
	STATUS_FLT_INTERNAL_ERROR:                                          "The Filter Manager had an internal error from which it cannot recover; therefore, the operation has failed. This is usually the result of a filter returning an invalid value from a pre-operation callback.",
	STATUS_FLT_DELETING_OBJECT:                                         "The object specified for this action is in the process of being deleted; therefore, the action requested cannot be completed at this time.",
	STATUS_FLT_MUST_BE_NONPAGED_POOL:                                   "A nonpaged pool must be used for this type of context.",
	STATUS_FLT_DUPLICATE_ENTRY:                                         "A duplicate handler definition has been provided for an operation.",
	STATUS_FLT_CBDQ_DISABLED:                                           "The callback data queue has been disabled.",
	STATUS_FLT_DO_NOT_ATTACH:                                           "Do not attach the filter to the volume at this time.",
	STATUS_FLT_DO_NOT_DETACH:                                           "Do not detach the filter from the volume at this time.",
	STATUS_FLT_INSTANCE_ALTITUDE_COLLISION:                             "An instance already exists at this altitude on the volume specified.",
	STATUS_FLT_INSTANCE_NAME_COLLISION:                                 "An instance already exists with this name on the volume specified.",
	STATUS_FLT_FILTER_NOT_FOUND:                                        "The system could not find the filter specified.",
	STATUS_FLT_VOLUME_NOT_FOUND:                                        "The system could not find the volume specified.",
	STATUS_FLT_INSTANCE_NOT_FOUND:                                      "The system could not find the instance specified.",
	STATUS_FLT_CONTEXT_ALLOCATION_NOT_FOUND:                            "No registered context allocation definition was found for the given request.",
	STATUS_FLT_INVALID_CONTEXT_REGISTRATION:                            "An invalid parameter was specified during context registration.",
	STATUS_FLT_NAME_CACHE_MISS:                                         "The name requested was not found in the Filter Manager name cache and could not be retrieved from the file system.",
	STATUS_FLT_NO_DEVICE_OBJECT:                                        "The requested device object does not exist for the given volume.",
	STATUS_FLT_VOLUME_ALREADY_MOUNTED:                                  "The specified volume is already mounted.",
	STATUS_FLT_ALREADY_ENLISTED:                                        "The specified transaction context is already enlisted in a transaction.",
	STATUS_FLT_CONTEXT_ALREADY_LINKED:                                  "The specified context is already attached to another object.",
	STATUS_FLT_NO_WAITER_FOR_REPLY:                                     "No waiter is present for the filter's reply to this message.",
	STATUS_MONITOR_NO_DESCRIPTOR:                                       "A monitor descriptor could not be obtained.",
	STATUS_MONITOR_UNKNOWN_DESCRIPTOR_FORMAT:                           "This release does not support the format of the obtained monitor descriptor.",
	STATUS_MONITOR_INVALID_DESCRIPTOR_CHECKSUM:                         "The checksum of the obtained monitor descriptor is invalid.",
	STATUS_MONITOR_INVALID_STANDARD_TIMING_BLOCK:                       "The monitor descriptor contains an invalid standard timing block.",
	STATUS_MONITOR_WMI_DATABLOCK_REGISTRATION_FAILED:                   "WMI data-block registration failed for one of the MSMonitorClass WMI subclasses.",
	STATUS_MONITOR_INVALID_SERIAL_NUMBER_MONDSC_BLOCK:                  "The provided monitor descriptor block is either corrupted or does not contain the monitor's detailed serial number.",
	STATUS_MONITOR_INVALID_USER_FRIENDLY_MONDSC_BLOCK:                  "The provided monitor descriptor block is either corrupted or does not contain the monitor's user-friendly name.",
	STATUS_MONITOR_NO_MORE_DESCRIPTOR_DATA:                             "There is no monitor descriptor data at the specified (offset or size) region.",
	STATUS_MONITOR_INVALID_DETAILED_TIMING_BLOCK:                       "The monitor descriptor contains an invalid detailed timing block.",
	STATUS_MONITOR_INVALID_MANUFACTURE_DATE:                            "Monitor descriptor contains invalid manufacture date.",
	STATUS_GRAPHICS_NOT_EXCLUSIVE_MODE_OWNER:                           "Exclusive mode ownership is needed to create an unmanaged primary allocation.",
	STATUS_GRAPHICS_INSUFFICIENT_DMA_BUFFER:                            "The driver needs more DMA buffer space to complete the requested operation.",
	STATUS_GRAPHICS_INVALID_DISPLAY_ADAPTER:                            "The specified display adapter handle is invalid.",
	STATUS_GRAPHICS_ADAPTER_WAS_RESET:                                  "The specified display adapter and all of its state have been reset.",
	STATUS_GRAPHICS_INVALID_DRIVER_MODEL:                               "The driver stack does not match the expected driver model.",
	STATUS_GRAPHICS_PRESENT_MODE_CHANGED:                               "Present happened but ended up into the changed desktop mode.",
	STATUS_GRAPHICS_PRESENT_OCCLUDED:                                   "Nothing to present due to desktop occlusion.",
	STATUS_GRAPHICS_PRESENT_DENIED:                                     "Not able to present due to denial of desktop access.",
	STATUS_GRAPHICS_CANNOTCOLORCONVERT:                                 "Not able to present with color conversion.",
	STATUS_GRAPHICS_PRESENT_REDIRECTION_DISABLED:                       "Present redirection is disabled (desktop windowing management subsystem is off).",
	STATUS_GRAPHICS_PRESENT_UNOCCLUDED:                                 "Previous exclusive VidPn source owner has released its ownership",
	STATUS_GRAPHICS_NO_VIDEO_MEMORY:                                    "Not enough video memory is available to complete the operation.",
	STATUS_GRAPHICS_CANT_LOCK_MEMORY:                                   "Could not probe and lock the underlying memory of an allocation.",
	STATUS_GRAPHICS_ALLOCATION_BUSY:                                    "The allocation is currently busy.",
	STATUS_GRAPHICS_TOO_MANY_REFERENCES:                                "An object being referenced has already reached the maximum reference count and cannot be referenced further.",
	STATUS_GRAPHICS_TRY_AGAIN_LATER:                                    "A problem could not be solved due to an existing condition. Try again later.",
	STATUS_GRAPHICS_TRY_AGAIN_NOW:                                      "A problem could not be solved due to an existing condition. Try again now.",
	STATUS_GRAPHICS_ALLOCATION_INVALID:                                 "The allocation is invalid.",
	STATUS_GRAPHICS_UNSWIZZLING_APERTURE_UNAVAILABLE:                   "No more unswizzling apertures are currently available.",
	STATUS_GRAPHICS_UNSWIZZLING_APERTURE_UNSUPPORTED:                   "The current allocation cannot be unswizzled by an aperture.",
	STATUS_GRAPHICS_CANT_EVICT_PINNED_ALLOCATION:                       "The request failed because a pinned allocation cannot be evicted.",
	STATUS_GRAPHICS_INVALID_ALLOCATION_USAGE:                           "The allocation cannot be used from its current segment location for the specified operation.",
	STATUS_GRAPHICS_CANT_RENDER_LOCKED_ALLOCATION:                      "A locked allocation cannot be used in the current command buffer.",
	STATUS_GRAPHICS_ALLOCATION_CLOSED:                                  "The allocation being referenced has been closed permanently.",
	STATUS_GRAPHICS_INVALID_ALLOCATION_INSTANCE:                        "An invalid allocation instance is being referenced.",
	STATUS_GRAPHICS_INVALID_ALLOCATION_HANDLE:                          "An invalid allocation handle is being referenced.",
	STATUS_GRAPHICS_WRONG_ALLOCATION_DEVICE:                            "The allocation being referenced does not belong to the current device.",
	STATUS_GRAPHICS_ALLOCATION_CONTENT_LOST:                            "The specified allocation lost its content.",
	STATUS_GRAPHICS_GPU_EXCEPTION_ON_DEVICE:                            "A GPU exception was detected on the given device. The device cannot be scheduled.",
	STATUS_GRAPHICS_INVALID_VIDPN_TOPOLOGY:                             "The specified VidPN topology is invalid.",
	STATUS_GRAPHICS_VIDPN_TOPOLOGY_NOT_SUPPORTED:                       "The specified VidPN topology is valid but is not supported by this model of the display adapter.",
	STATUS_GRAPHICS_VIDPN_TOPOLOGY_CURRENTLY_NOT_SUPPORTED:             "The specified VidPN topology is valid but is not currently supported by the display adapter due to allocation of its resources.",
	STATUS_GRAPHICS_INVALID_VIDPN:                                      "The specified VidPN handle is invalid.",
	STATUS_GRAPHICS_INVALID_VIDEO_PRESENT_SOURCE:                       "The specified video present source is invalid.",
	STATUS_GRAPHICS_INVALID_VIDEO_PRESENT_TARGET:                       "The specified video present target is invalid.",
	STATUS_GRAPHICS_VIDPN_MODALITY_NOT_SUPPORTED:                       "The specified VidPN modality is not supported (for example, at least two of the pinned modes are not co-functional).",
	STATUS_GRAPHICS_INVALID_VIDPN_SOURCEMODESET:                        "The specified VidPN source mode set is invalid.",
	STATUS_GRAPHICS_INVALID_VIDPN_TARGETMODESET:                        "The specified VidPN target mode set is invalid.",
	STATUS_GRAPHICS_INVALID_FREQUENCY:                                  "The specified video signal frequency is invalid.",
	STATUS_GRAPHICS_INVALID_ACTIVE_REGION:                              "The specified video signal active region is invalid.",
	STATUS_GRAPHICS_INVALID_TOTAL_REGION:                               "The specified video signal total region is invalid.",
	STATUS_GRAPHICS_INVALID_VIDEO_PRESENT_SOURCE_MODE:                  "The specified video present source mode is invalid.",
	STATUS_GRAPHICS_INVALID_VIDEO_PRESENT_TARGET_MODE:                  "The specified video present target mode is invalid.",
	STATUS_GRAPHICS_PINNED_MODE_MUST_REMAIN_IN_SET:                     "The pinned mode must remain in the set on the VidPN's co-functional modality enumeration.",
	STATUS_GRAPHICS_PATH_ALREADY_IN_TOPOLOGY:                           "The specified video present path is already in the VidPN's topology.",
	STATUS_GRAPHICS_MODE_ALREADY_IN_MODESET:                            "The specified mode is already in the mode set.",
	STATUS_GRAPHICS_INVALID_VIDEOPRESENTSOURCESET:                      "The specified video present source set is invalid.",
	STATUS_GRAPHICS_INVALID_VIDEOPRESENTTARGETSET:                      "The specified video present target set is invalid.",
	STATUS_GRAPHICS_SOURCE_ALREADY_IN_SET:                              "The specified video present source is already in the video present source set.",
	STATUS_GRAPHICS_TARGET_ALREADY_IN_SET:                              "The specified video present target is already in the video present target set.",
	STATUS_GRAPHICS_INVALID_VIDPN_PRESENT_PATH:                         "The specified VidPN present path is invalid.",
	STATUS_GRAPHICS_NO_RECOMMENDED_VIDPN_TOPOLOGY:                      "The miniport has no recommendation for augmenting the specified VidPN's topology.",
	STATUS_GRAPHICS_INVALID_MONITOR_FREQUENCYRANGESET:                  "The specified monitor frequency range set is invalid.",
	STATUS_GRAPHICS_INVALID_MONITOR_FREQUENCYRANGE:                     "The specified monitor frequency range is invalid.",
	STATUS_GRAPHICS_FREQUENCYRANGE_NOT_IN_SET:                          "The specified frequency range is not in the specified monitor frequency range set.",
	STATUS_GRAPHICS_FREQUENCYRANGE_ALREADY_IN_SET:                      "The specified frequency range is already in the specified monitor frequency range set.",
	STATUS_GRAPHICS_STALE_MODESET:                                      "The specified mode set is stale. Reacquire the new mode set.",
	STATUS_GRAPHICS_INVALID_MONITOR_SOURCEMODESET:                      "The specified monitor source mode set is invalid.",
	STATUS_GRAPHICS_INVALID_MONITOR_SOURCE_MODE:                        "The specified monitor source mode is invalid.",
	STATUS_GRAPHICS_NO_RECOMMENDED_FUNCTIONAL_VIDPN:                    "The miniport does not have a recommendation regarding the request to provide a functional VidPN given the current display adapter configuration.",
	STATUS_GRAPHICS_MODE_ID_MUST_BE_UNIQUE:                             "The ID of the specified mode is being used by another mode in the set.",
	STATUS_GRAPHICS_EMPTY_ADAPTER_MONITOR_MODE_SUPPORT_INTERSECTION:    "The system failed to determine a mode that is supported by both the display adapter and the monitor connected to it.",
	STATUS_GRAPHICS_VIDEO_PRESENT_TARGETS_LESS_THAN_SOURCES:            "The number of video present targets must be greater than or equal to the number of video present sources.",
	STATUS_GRAPHICS_PATH_NOT_IN_TOPOLOGY:                               "The specified present path is not in the VidPN's topology.",
	STATUS_GRAPHICS_ADAPTER_MUST_HAVE_AT_LEAST_ONE_SOURCE:              "The display adapter must have at least one video present source.",
	STATUS_GRAPHICS_ADAPTER_MUST_HAVE_AT_LEAST_ONE_TARGET:              "The display adapter must have at least one video present target.",
	STATUS_GRAPHICS_INVALID_MONITORDESCRIPTORSET:                       "The specified monitor descriptor set is invalid.",
	STATUS_GRAPHICS_INVALID_MONITORDESCRIPTOR:                          "The specified monitor descriptor is invalid.",
	STATUS_GRAPHICS_MONITORDESCRIPTOR_NOT_IN_SET:                       "The specified descriptor is not in the specified monitor descriptor set.",
	STATUS_GRAPHICS_MONITORDESCRIPTOR_ALREADY_IN_SET:                   "The specified descriptor is already in the specified monitor descriptor set.",
	STATUS_GRAPHICS_MONITORDESCRIPTOR_ID_MUST_BE_UNIQUE:                "The ID of the specified monitor descriptor is being used by another descriptor in the set.",
	STATUS_GRAPHICS_INVALID_VIDPN_TARGET_SUBSET_TYPE:                   "The specified video present target subset type is invalid.",
	STATUS_GRAPHICS_RESOURCES_NOT_RELATED:                              "Two or more of the specified resources are not related to each other, as defined by the interface semantics.",
	STATUS_GRAPHICS_SOURCE_ID_MUST_BE_UNIQUE:                           "The ID of the specified video present source is being used by another source in the set.",
	STATUS_GRAPHICS_TARGET_ID_MUST_BE_UNIQUE:                           "The ID of the specified video present target is being used by another target in the set.",
	STATUS_GRAPHICS_NO_AVAILABLE_VIDPN_TARGET:                          "The specified VidPN source cannot be used because there is no available VidPN target to connect it to.",
	STATUS_GRAPHICS_MONITOR_COULD_NOT_BE_ASSOCIATED_WITH_ADAPTER:       "The newly arrived monitor could not be associated with a display adapter.",
	STATUS_GRAPHICS_NO_VIDPNMGR:                                        "The particular display adapter does not have an associated VidPN manager.",
	STATUS_GRAPHICS_NO_ACTIVE_VIDPN:                                    "The VidPN manager of the particular display adapter does not have an active VidPN.",
	STATUS_GRAPHICS_STALE_VIDPN_TOPOLOGY:                               "The specified VidPN topology is stale; obtain the new topology.",
	STATUS_GRAPHICS_MONITOR_NOT_CONNECTED:                              "No monitor is connected on the specified video present target.",
	STATUS_GRAPHICS_SOURCE_NOT_IN_TOPOLOGY:                             "The specified source is not part of the specified VidPN's topology.",
	STATUS_GRAPHICS_INVALID_PRIMARYSURFACE_SIZE:                        "The specified primary surface size is invalid.",
	STATUS_GRAPHICS_INVALID_VISIBLEREGION_SIZE:                         "The specified visible region size is invalid.",
	STATUS_GRAPHICS_INVALID_STRIDE:                                     "The specified stride is invalid.",
	STATUS_GRAPHICS_INVALID_PIXELFORMAT:                                "The specified pixel format is invalid.",
	STATUS_GRAPHICS_INVALID_COLORBASIS:                                 "The specified color basis is invalid.",
	STATUS_GRAPHICS_INVALID_PIXELVALUEACCESSMODE:                       "The specified pixel value access mode is invalid.",
	STATUS_GRAPHICS_TARGET_NOT_IN_TOPOLOGY:                             "The specified target is not part of the specified VidPN's topology.",
	STATUS_GRAPHICS_NO_DISPLAY_MODE_MANAGEMENT_SUPPORT:                 "Failed to acquire the display mode management interface.",
	STATUS_GRAPHICS_VIDPN_SOURCE_IN_USE:                                "The specified VidPN source is already owned by a DMM client and cannot be used until that client releases it.",
	STATUS_GRAPHICS_CANT_ACCESS_ACTIVE_VIDPN:                           "The specified VidPN is active and cannot be accessed.",
	STATUS_GRAPHICS_INVALID_PATH_IMPORTANCE_ORDINAL:                    "The specified VidPN's present path importance ordinal is invalid.",
	STATUS_GRAPHICS_INVALID_PATH_CONTENT_GEOMETRY_TRANSFORMATION:       "The specified VidPN's present path content geometry transformation is invalid.",
	STATUS_GRAPHICS_PATH_CONTENT_GEOMETRY_TRANSFORMATION_NOT_SUPPORTED: "The specified content geometry transformation is not supported on the respective VidPN present path.",
	STATUS_GRAPHICS_INVALID_GAMMA_RAMP:                                 "The specified gamma ramp is invalid.",
	STATUS_GRAPHICS_GAMMA_RAMP_NOT_SUPPORTED:                           "The specified gamma ramp is not supported on the respective VidPN present path.",
	STATUS_GRAPHICS_MULTISAMPLING_NOT_SUPPORTED:                        "Multisampling is not supported on the respective VidPN present path.",
	STATUS_GRAPHICS_MODE_NOT_IN_MODESET:                                "The specified mode is not in the specified mode set.",
	STATUS_GRAPHICS_INVALID_VIDPN_TOPOLOGY_RECOMMENDATION_REASON:       "The specified VidPN topology recommendation reason is invalid.",
	STATUS_GRAPHICS_INVALID_PATH_CONTENT_TYPE:                          "The specified VidPN present path content type is invalid.",
	STATUS_GRAPHICS_INVALID_COPYPROTECTION_TYPE:                        "The specified VidPN present path copy protection type is invalid.",
	STATUS_GRAPHICS_UNASSIGNED_MODESET_ALREADY_EXISTS:                  "Only one unassigned mode set can exist at any one time for a particular VidPN source or target.",
	STATUS_GRAPHICS_INVALID_SCANLINE_ORDERING:                          "The specified scan line ordering type is invalid.",
	STATUS_GRAPHICS_TOPOLOGY_CHANGES_NOT_ALLOWED:                       "The topology changes are not allowed for the specified VidPN.",
	STATUS_GRAPHICS_NO_AVAILABLE_IMPORTANCE_ORDINALS:                   "All available importance ordinals are being used in the specified topology.",
	STATUS_GRAPHICS_INCOMPATIBLE_PRIVATE_FORMAT:                        "The specified primary surface has a different private-format attribute than the current primary surface.",
	STATUS_GRAPHICS_INVALID_MODE_PRUNING_ALGORITHM:                     "The specified mode-pruning algorithm is invalid.",
	STATUS_GRAPHICS_INVALID_MONITOR_CAPABILITY_ORIGIN:                  "The specified monitor-capability origin is invalid.",
	STATUS_GRAPHICS_INVALID_MONITOR_FREQUENCYRANGE_CONSTRAINT:          "The specified monitor-frequency range constraint is invalid.",
	STATUS_GRAPHICS_MAX_NUM_PATHS_REACHED:                              "The maximum supported number of present paths has been reached.",
	STATUS_GRAPHICS_CANCEL_VIDPN_TOPOLOGY_AUGMENTATION:                 "The miniport requested that augmentation be canceled for the specified source of the specified VidPN's topology.",
	STATUS_GRAPHICS_INVALID_CLIENT_TYPE:                                "The specified client type was not recognized.",
	STATUS_GRAPHICS_CLIENTVIDPN_NOT_SET:                                "The client VidPN is not set on this adapter (for example, no user mode-initiated mode changes have taken place on this adapter).",
	STATUS_GRAPHICS_SPECIFIED_CHILD_ALREADY_CONNECTED:                  "The specified display adapter child device already has an external device connected to it.",
	STATUS_GRAPHICS_CHILD_DESCRIPTOR_NOT_SUPPORTED:                     "The display adapter child device does not support reporting a descriptor.",
	STATUS_GRAPHICS_NOT_A_LINKED_ADAPTER:                               "The display adapter is not linked to any other adapters.",
	STATUS_GRAPHICS_LEADLINK_NOT_ENUMERATED:                            "The lead adapter in a linked configuration was not enumerated yet.",
	STATUS_GRAPHICS_CHAINLINKS_NOT_ENUMERATED:                          "Some chain adapters in a linked configuration have not yet been enumerated.",
	STATUS_GRAPHICS_ADAPTER_CHAIN_NOT_READY:                            "The chain of linked adapters is not ready to start because of an unknown failure.",
	STATUS_GRAPHICS_CHAINLINKS_NOT_STARTED:                             "An attempt was made to start a lead link display adapter when the chain links had not yet started.",
	STATUS_GRAPHICS_CHAINLINKS_NOT_POWERED_ON:                          "An attempt was made to turn on a lead link display adapter when the chain links were turned off.",
	STATUS_GRAPHICS_INCONSISTENT_DEVICE_LINK_STATE:                     "The adapter link was found in an inconsistent state. Not all adapters are in an expected PNP/power state.",
	STATUS_GRAPHICS_NOT_POST_DEVICE_DRIVER:                             "The driver trying to start is not the same as the driver for the posted display adapter.",
	STATUS_GRAPHICS_ADAPTER_ACCESS_NOT_EXCLUDED:                        "An operation is being attempted that requires the display adapter to be in a quiescent state.",
	STATUS_GRAPHICS_OPM_NOT_SUPPORTED:                                  "The driver does not support OPM.",
	STATUS_GRAPHICS_COPP_NOT_SUPPORTED:                                 "The driver does not support COPP.",
	STATUS_GRAPHICS_UAB_NOT_SUPPORTED:                                  "The driver does not support UAB.",
	STATUS_GRAPHICS_OPM_INVALID_ENCRYPTED_PARAMETERS:                   "The specified encrypted parameters are invalid.",
	STATUS_GRAPHICS_OPM_PARAMETER_ARRAY_TOO_SMALL:                      "An array passed to a function cannot hold all of the data that the function wants to put in it.",
	STATUS_GRAPHICS_OPM_NO_PROTECTED_OUTPUTS_EXIST:                     "The GDI display device passed to this function does not have any active protected outputs.",
	STATUS_GRAPHICS_PVP_NO_DISPLAY_DEVICE_CORRESPONDS_TO_NAME:          "The PVP cannot find an actual GDI display device that corresponds to the passed-in GDI display device name.",
	STATUS_GRAPHICS_PVP_DISPLAY_DEVICE_NOT_ATTACHED_TO_DESKTOP:         "This function failed because the GDI display device passed to it was not attached to the Windows desktop.",
	STATUS_GRAPHICS_PVP_MIRRORING_DEVICES_NOT_SUPPORTED:                "The PVP does not support mirroring display devices because they do not have any protected outputs.",
	STATUS_GRAPHICS_OPM_INVALID_POINTER:                                "The function failed because an invalid pointer parameter was passed to it. A pointer parameter is invalid if it is null, is not correctly aligned, or it points to an invalid address or a kernel mode address.",
	STATUS_GRAPHICS_OPM_INTERNAL_ERROR:                                 "An internal error caused an operation to fail.",
	STATUS_GRAPHICS_OPM_INVALID_HANDLE:                                 "The function failed because the caller passed in an invalid OPM user-mode handle.",
	STATUS_GRAPHICS_PVP_NO_MONITORS_CORRESPOND_TO_DISPLAY_DEVICE:       "This function failed because the GDI device passed to it did not have any monitors associated with it.",
	STATUS_GRAPHICS_PVP_INVALID_CERTIFICATE_LENGTH:                     "A certificate could not be returned because the certificate buffer passed to the function was too small.",
	STATUS_GRAPHICS_OPM_SPANNING_MODE_ENABLED:                          "DxgkDdiOpmCreateProtectedOutput() could not create a protected output because the video present yarget is in spanning mode.",
	STATUS_GRAPHICS_OPM_THEATER_MODE_ENABLED:                           "DxgkDdiOpmCreateProtectedOutput() could not create a protected output because the video present target is in theater mode.",
	STATUS_GRAPHICS_PVP_HFS_FAILED:                                     "The function call failed because the display adapter's hardware functionality scan (HFS) failed to validate the graphics hardware.",
	STATUS_GRAPHICS_OPM_INVALID_SRM:                                    "The HDCP SRM passed to this function did not comply with section 5 of the HDCP 1.1 specification.",
	STATUS_GRAPHICS_OPM_OUTPUT_DOES_NOT_SUPPORT_HDCP:                   "The protected output cannot enable the HDCP system because it does not support it.",
	STATUS_GRAPHICS_OPM_OUTPUT_DOES_NOT_SUPPORT_ACP:                    "The protected output cannot enable analog copy protection because it does not support it.",
	STATUS_GRAPHICS_OPM_OUTPUT_DOES_NOT_SUPPORT_CGMSA:                  "The protected output cannot enable the CGMS-A protection technology because it does not support it.",
	STATUS_GRAPHICS_OPM_HDCP_SRM_NEVER_SET:                             "DxgkDdiOPMGetInformation() cannot return the version of the SRM being used because the application never successfully passed an SRM to the protected output.",
	STATUS_GRAPHICS_OPM_RESOLUTION_TOO_HIGH:                            "DxgkDdiOPMConfigureProtectedOutput() cannot enable the specified output protection technology because the output's screen resolution is too high.",
	STATUS_GRAPHICS_OPM_ALL_HDCP_HARDWARE_ALREADY_IN_USE:               "DxgkDdiOPMConfigureProtectedOutput() cannot enable HDCP because other physical outputs are using the display adapter's HDCP hardware.",
	STATUS_GRAPHICS_OPM_PROTECTED_OUTPUT_NO_LONGER_EXISTS:              "The operating system asynchronously destroyed this OPM-protected output because the operating system state changed. This error typically occurs because the monitor PDO associated with this protected output was removed or stopped, the protected output's session became a nonconsole session, or the protected output's desktop became inactive.",
	STATUS_GRAPHICS_OPM_SESSION_TYPE_CHANGE_IN_PROGRESS:                "OPM functions cannot be called when a session is changing its type. Three types of sessions currently exist: console, disconnected, and remote (RDP or ICA).",
	STATUS_GRAPHICS_OPM_PROTECTED_OUTPUT_DOES_NOT_HAVE_COPP_SEMANTICS:  "The DxgkDdiOPMGetCOPPCompatibleInformation, DxgkDdiOPMGetInformation, or DxgkDdiOPMConfigureProtectedOutput function failed. This error is returned only if a protected output has OPM semantics. DxgkDdiOPMGetCOPPCompatibleInformation always returns this error if a protected output has OPM semantics.DxgkDdiOPMGetInformation returns this error code if the caller requested COPP-specific information.DxgkDdiOPMConfigureProtectedOutput returns this error when the caller tries to use a COPP-specific command.",
	STATUS_GRAPHICS_OPM_INVALID_INFORMATION_REQUEST:                    "The DxgkDdiOPMGetInformation and DxgkDdiOPMGetCOPPCompatibleInformation functions return this error code if the passed-in sequence number is not the expected sequence number or the passed-in OMAC value is invalid.",
	STATUS_GRAPHICS_OPM_DRIVER_INTERNAL_ERROR:                          "The function failed because an unexpected error occurred inside a display driver.",
	STATUS_GRAPHICS_OPM_PROTECTED_OUTPUT_DOES_NOT_HAVE_OPM_SEMANTICS:   "The DxgkDdiOPMGetCOPPCompatibleInformation, DxgkDdiOPMGetInformation, or DxgkDdiOPMConfigureProtectedOutput function failed. This error is returned only if a protected output has COPP semantics. DxgkDdiOPMGetCOPPCompatibleInformation returns this error code if the caller requested OPM-specific information.DxgkDdiOPMGetInformation always returns this error if a protected output has COPP semantics.DxgkDdiOPMConfigureProtectedOutput returns this error when the caller tries to use an OPM-specific command.",
	STATUS_GRAPHICS_OPM_SIGNALING_NOT_SUPPORTED:                        "The DxgkDdiOPMGetCOPPCompatibleInformation and DxgkDdiOPMConfigureProtectedOutput functions return this error if the display driver does not support the DXGKMDT_OPM_GET_ACP_AND_CGMSA_SIGNALING and DXGKMDT_OPM_SET_ACP_AND_CGMSA_SIGNALING GUIDs.",
	STATUS_GRAPHICS_OPM_INVALID_CONFIGURATION_REQUEST:                  "The DxgkDdiOPMConfigureProtectedOutput function returns this error code if the passed-in sequence number is not the expected sequence number or the passed-in OMAC value is invalid.",
	STATUS_GRAPHICS_I2C_NOT_SUPPORTED:                                  "The monitor connected to the specified video output does not have an I2C bus.",
	STATUS_GRAPHICS_I2C_DEVICE_DOES_NOT_EXIST:                          "No device on the I2C bus has the specified address.",
	STATUS_GRAPHICS_I2C_ERROR_TRANSMITTING_DATA:                        "An error occurred while transmitting data to the device on the I2C bus.",
	STATUS_GRAPHICS_I2C_ERROR_RECEIVING_DATA:                           "An error occurred while receiving data from the device on the I2C bus.",
	STATUS_GRAPHICS_DDCCI_VCP_NOT_SUPPORTED:                            "The monitor does not support the specified VCP code.",
	STATUS_GRAPHICS_DDCCI_INVALID_DATA:                                 "The data received from the monitor is invalid.",
	STATUS_GRAPHICS_DDCCI_MONITOR_RETURNED_INVALID_TIMING_STATUS_BYTE:  "A function call failed because a monitor returned an invalid timing status byte when the operating system used the DDC/CI get timing report and timing message command to get a timing report from a monitor.",
	STATUS_GRAPHICS_DDCCI_INVALID_CAPABILITIES_STRING:                  "A monitor returned a DDC/CI capabilities string that did not comply with the ACCESS.bus 3.0, DDC/CI 1.1, or MCCS 2 Revision 1 specification.",
	STATUS_GRAPHICS_MCA_INTERNAL_ERROR:                                 "An internal error caused an operation to fail.",
	STATUS_GRAPHICS_DDCCI_INVALID_MESSAGE_COMMAND:                      "An operation failed because a DDC/CI message had an invalid value in its command field.",
	STATUS_GRAPHICS_DDCCI_INVALID_MESSAGE_LENGTH:                       "This error occurred because a DDC/CI message had an invalid value in its length field.",
	STATUS_GRAPHICS_DDCCI_INVALID_MESSAGE_CHECKSUM:                     "This error occurred because the value in a DDC/CI message's checksum field did not match the message's computed checksum value. This error implies that the data was corrupted while it was being transmitted from a monitor to a computer.",
	STATUS_GRAPHICS_INVALID_PHYSICAL_MONITOR_HANDLE:                    "This function failed because an invalid monitor handle was passed to it.",
	STATUS_GRAPHICS_MONITOR_NO_LONGER_EXISTS:                           "The operating system asynchronously destroyed the monitor that corresponds to this handle because the operating system's state changed. This error typically occurs because the monitor PDO associated with this handle was removed or stopped, or a display mode change occurred. A display mode change occurs when Windows sends a WM_DISPLAYCHANGE message to applications.",
	STATUS_GRAPHICS_ONLY_CONSOLE_SESSION_SUPPORTED:                     "This function can be used only if a program is running in the local console session. It cannot be used if a program is running on a remote desktop session or on a terminal server session.",
	STATUS_GRAPHICS_NO_DISPLAY_DEVICE_CORRESPONDS_TO_NAME:              "This function cannot find an actual GDI display device that corresponds to the specified GDI display device name.",
	STATUS_GRAPHICS_DISPLAY_DEVICE_NOT_ATTACHED_TO_DESKTOP:             "The function failed because the specified GDI display device was not attached to the Windows desktop.",
	STATUS_GRAPHICS_MIRRORING_DEVICES_NOT_SUPPORTED:                    "This function does not support GDI mirroring display devices because GDI mirroring display devices do not have any physical monitors associated with them.",
	STATUS_GRAPHICS_INVALID_POINTER:                                    "The function failed because an invalid pointer parameter was passed to it. A pointer parameter is invalid if it is null, is not correctly aligned, or points to an invalid address or to a kernel mode address.",
	STATUS_GRAPHICS_NO_MONITORS_CORRESPOND_TO_DISPLAY_DEVICE:           "This function failed because the GDI device passed to it did not have a monitor associated with it.",
	STATUS_GRAPHICS_PARAMETER_ARRAY_TOO_SMALL:                          "An array passed to the function cannot hold all of the data that the function must copy into the array.",
	STATUS_GRAPHICS_INTERNAL_ERROR:                                     "An internal error caused an operation to fail.",
	STATUS_GRAPHICS_SESSION_TYPE_CHANGE_IN_PROGRESS:                    "The function failed because the current session is changing its type. This function cannot be called when the current session is changing its type. Three types of sessions currently exist: console, disconnected, and remote (RDP or ICA).",
	STATUS_FVE_LOCKED_VOLUME:                                           "The volume must be unlocked before it can be used.",
	STATUS_FVE_NOT_ENCRYPTED:                                           "The volume is fully decrypted and no key is available.",
	STATUS_FVE_BAD_INFORMATION:                                         "The control block for the encrypted volume is not valid.",
	STATUS_FVE_TOO_SMALL:                                               "Not enough free space remains on the volume to allow encryption.",
	STATUS_FVE_FAILED_WRONG_FS:                                         "The partition cannot be encrypted because the file system is not supported.",
	STATUS_FVE_FAILED_BAD_FS:                                           "The file system is inconsistent. Run the Check Disk utility.",
	STATUS_FVE_FS_NOT_EXTENDED:                                         "The file system does not extend to the end of the volume.",
	STATUS_FVE_FS_MOUNTED:                                              "This operation cannot be performed while a file system is mounted on the volume.",
	STATUS_FVE_NO_LICENSE:                                              "BitLocker Drive Encryption is not included with this version of Windows.",
	STATUS_FVE_ACTION_NOT_ALLOWED:                                      "The requested action was denied by the FVE control engine.",
	STATUS_FVE_BAD_DATA:                                                "The data supplied is malformed.",
	STATUS_FVE_VOLUME_NOT_BOUND:                                        "The volume is not bound to the system.",
	STATUS_FVE_NOT_DATA_VOLUME:                                         "The volume specified is not a data volume.",
	STATUS_FVE_CONV_READ_ERROR:                                         "A read operation failed while converting the volume.",
	STATUS_FVE_CONV_WRITE_ERROR:                                        "A write operation failed while converting the volume.",
	STATUS_FVE_OVERLAPPED_UPDATE:                                       "The control block for the encrypted volume was updated by another thread. Try again.",
	STATUS_FVE_FAILED_SECTOR_SIZE:                                      "The volume encryption algorithm cannot be used on this sector size.",
	STATUS_FVE_FAILED_AUTHENTICATION:                                   "BitLocker recovery authentication failed.",
	STATUS_FVE_NOT_OS_VOLUME:                                           "The volume specified is not the boot operating system volume.",
	STATUS_FVE_KEYFILE_NOT_FOUND:                                       "The BitLocker startup key or recovery password could not be read from external media.",
	STATUS_FVE_KEYFILE_INVALID:                                         "The BitLocker startup key or recovery password file is corrupt or invalid.",
	STATUS_FVE_KEYFILE_NO_VMK:                                          "The BitLocker encryption key could not be obtained from the startup key or the recovery password.",
	STATUS_FVE_TPM_DISABLED:                                            "The TPM is disabled.",
	STATUS_FVE_TPM_SRK_AUTH_NOT_ZERO:                                   "The authorization data for the SRK of the TPM is not zero.",
	STATUS_FVE_TPM_INVALID_PCR:                                         "The system boot information changed or the TPM locked out access to BitLocker encryption keys until the computer is restarted.",
	STATUS_FVE_TPM_NO_VMK:                                              "The BitLocker encryption key could not be obtained from the TPM.",
	STATUS_FVE_PIN_INVALID:                                             "The BitLocker encryption key could not be obtained from the TPM and PIN.",
	STATUS_FVE_AUTH_INVALID_APPLICATION:                                "A boot application hash does not match the hash computed when BitLocker was turned on.",
	STATUS_FVE_AUTH_INVALID_CONFIG:                                     "The Boot Configuration Data (BCD) settings are not supported or have changed because BitLocker was enabled.",
	STATUS_FVE_DEBUGGER_ENABLED:                                        "Boot debugging is enabled. Run Windows Boot Configuration Data Store Editor (bcdedit.exe) to turn it off.",
	STATUS_FVE_DRY_RUN_FAILED:                                          "The BitLocker encryption key could not be obtained.",
	STATUS_FVE_BAD_METADATA_POINTER:                                    "The metadata disk region pointer is incorrect.",
	STATUS_FVE_OLD_METADATA_COPY:                                       "The backup copy of the metadata is out of date.",
	STATUS_FVE_REBOOT_REQUIRED:                                         "No action was taken because a system restart is required.",
	STATUS_FVE_RAW_ACCESS:                                              "No action was taken because BitLocker Drive Encryption is in RAW access mode.",
	STATUS_FVE_RAW_BLOCKED:                                             "BitLocker Drive Encryption cannot enter RAW access mode for this volume.",
	STATUS_FVE_NO_FEATURE_LICENSE:                                      "This feature of BitLocker Drive Encryption is not included with this version of Windows.",
	STATUS_FVE_POLICY_USER_DISABLE_RDV_NOT_ALLOWED:                     "Group policy does not permit turning off BitLocker Drive Encryption on roaming data volumes.",
	STATUS_FVE_CONV_RECOVERY_FAILED:                                    "Bitlocker Drive Encryption failed to recover from aborted conversion. This could be due to either all conversion logs being corrupted or the media being write-protected.",
	STATUS_FVE_VIRTUALIZED_SPACE_TOO_BIG:                               "The requested virtualization size is too big.",
	STATUS_FVE_VOLUME_TOO_SMALL:                                        "The drive is too small to be protected using BitLocker Drive Encryption.",
	STATUS_FWP_CALLOUT_NOT_FOUND:                                       "The callout does not exist.",
	STATUS_FWP_CONDITION_NOT_FOUND:                                     "The filter condition does not exist.",
	STATUS_FWP_FILTER_NOT_FOUND:                                        "The filter does not exist.",
	STATUS_FWP_LAYER_NOT_FOUND:                                         "The layer does not exist.",
	STATUS_FWP_PROVIDER_NOT_FOUND:                                      "The provider does not exist.",
	STATUS_FWP_PROVIDER_CONTEXT_NOT_FOUND:                              "The provider context does not exist.",
	STATUS_FWP_SUBLAYER_NOT_FOUND:                                      "The sublayer does not exist.",
	STATUS_FWP_NOT_FOUND:                                               "The object does not exist.",
	STATUS_FWP_ALREADY_EXISTS:                                          "An object with that GUID or LUID already exists.",
	STATUS_FWP_IN_USE:                                                  "The object is referenced by other objects and cannot be deleted.",
	STATUS_FWP_DYNAMIC_SESSION_IN_PROGRESS:                             "The call is not allowed from within a dynamic session.",
	STATUS_FWP_WRONG_SESSION:                                           "The call was made from the wrong session and cannot be completed.",
	STATUS_FWP_NO_TXN_IN_PROGRESS:                                      "The call must be made from within an explicit transaction.",
	STATUS_FWP_TXN_IN_PROGRESS:                                         "The call is not allowed from within an explicit transaction.",
	STATUS_FWP_TXN_ABORTED:                                             "The explicit transaction has been forcibly canceled.",
	STATUS_FWP_SESSION_ABORTED:                                         "The session has been canceled.",
	STATUS_FWP_INCOMPATIBLE_TXN:                                        "The call is not allowed from within a read-only transaction.",
	STATUS_FWP_TIMEOUT:                                                 "The call timed out while waiting to acquire the transaction lock.",
	STATUS_FWP_NET_EVENTS_DISABLED:                                     "The collection of network diagnostic events is disabled.",
	STATUS_FWP_INCOMPATIBLE_LAYER:                                      "The operation is not supported by the specified layer.",
	STATUS_FWP_KM_CLIENTS_ONLY:                                         "The call is allowed for kernel-mode callers only.",
	STATUS_FWP_LIFETIME_MISMATCH:                                       "The call tried to associate two objects with incompatible lifetimes.",
	STATUS_FWP_BUILTIN_OBJECT:                                          "The object is built-in and cannot be deleted.",
	STATUS_FWP_TOO_MANY_BOOTTIME_FILTERS:                               "The maximum number of boot-time filters has been reached.",
	STATUS_FWP_NOTIFICATION_DROPPED:                                    "A notification could not be delivered because a message queue has reached maximum capacity.",
	STATUS_FWP_TRAFFIC_MISMATCH:                                        "The traffic parameters do not match those for the security association context.",
	STATUS_FWP_INCOMPATIBLE_SA_STATE:                                   "The call is not allowed for the current security association state.",
	STATUS_FWP_NULL_POINTER:                                            "A required pointer is null.",
	STATUS_FWP_INVALID_ENUMERATOR:                                      "An enumerator is not valid.",
	STATUS_FWP_INVALID_FLAGS:                                           "The flags field contains an invalid value.",
	STATUS_FWP_INVALID_NET_MASK:                                        "A network mask is not valid.",
	STATUS_FWP_INVALID_RANGE:                                           "An FWP_RANGE is not valid.",
	STATUS_FWP_INVALID_INTERVAL:                                        "The time interval is not valid.",
	STATUS_FWP_ZERO_LENGTH_ARRAY:                                       "An array that must contain at least one element has a zero length.",
	STATUS_FWP_NULL_DISPLAY_NAME:                                       "The displayData.name field cannot be null.",
	STATUS_FWP_INVALID_ACTION_TYPE:                                     "The action type is not one of the allowed action types for a filter.",
	STATUS_FWP_INVALID_WEIGHT:                                          "The filter weight is not valid.",
	STATUS_FWP_MATCH_TYPE_MISMATCH:                                     "A filter condition contains a match type that is not compatible with the operands.",
	STATUS_FWP_TYPE_MISMATCH:                                           "An FWP_VALUE or FWPM_CONDITION_VALUE is of the wrong type.",
	STATUS_FWP_OUT_OF_BOUNDS:                                           "An integer value is outside the allowed range.",
	STATUS_FWP_RESERVED:                                                "A reserved field is nonzero.",
	STATUS_FWP_DUPLICATE_CONDITION:                                     "A filter cannot contain multiple conditions operating on a single field.",
	STATUS_FWP_DUPLICATE_KEYMOD:                                        "A policy cannot contain the same keying module more than once.",
	STATUS_FWP_ACTION_INCOMPATIBLE_WITH_LAYER:                          "The action type is not compatible with the layer.",
	STATUS_FWP_ACTION_INCOMPATIBLE_WITH_SUBLAYER:                       "The action type is not compatible with the sublayer.",
	STATUS_FWP_CONTEXT_INCOMPATIBLE_WITH_LAYER:                         "The raw context or the provider context is not compatible with the layer.",
	STATUS_FWP_CONTEXT_INCOMPATIBLE_WITH_CALLOUT:                       "The raw context or the provider context is not compatible with the callout.",
	STATUS_FWP_INCOMPATIBLE_AUTH_METHOD:                                "The authentication method is not compatible with the policy type.",
	STATUS_FWP_INCOMPATIBLE_DH_GROUP:                                   "The Diffie-Hellman group is not compatible with the policy type.",
	STATUS_FWP_EM_NOT_SUPPORTED:                                        "An IKE policy cannot contain an Extended Mode policy.",
	STATUS_FWP_NEVER_MATCH:                                             "The enumeration template or subscription will never match any objects.",
	STATUS_FWP_PROVIDER_CONTEXT_MISMATCH:                               "The provider context is of the wrong type.",
	STATUS_FWP_INVALID_PARAMETER:                                       "The parameter is incorrect.",
	STATUS_FWP_TOO_MANY_SUBLAYERS:                                      "The maximum number of sublayers has been reached.",
	STATUS_FWP_CALLOUT_NOTIFICATION_FAILED:                             "The notification function for a callout returned an error.",
	STATUS_FWP_INCOMPATIBLE_AUTH_CONFIG:                                "The IPsec authentication configuration is not compatible with the authentication type.",
	STATUS_FWP_INCOMPATIBLE_CIPHER_CONFIG:                              "The IPsec cipher configuration is not compatible with the cipher type.",
	STATUS_FWP_DUPLICATE_AUTH_METHOD:                                   "A policy cannot contain the same auth method more than once.",
	STATUS_FWP_TCPIP_NOT_READY:                                         "The TCP/IP stack is not ready.",
	STATUS_FWP_INJECT_HANDLE_CLOSING:                                   "The injection handle is being closed by another thread.",
	STATUS_FWP_INJECT_HANDLE_STALE:                                     "The injection handle is stale.",
	STATUS_FWP_CANNOT_PEND:                                             "The classify cannot be pended.",
	STATUS_NDIS_CLOSING:                                                "The binding to the network interface is being closed.",
	STATUS_NDIS_BAD_VERSION:                                            "An invalid version was specified.",
	STATUS_NDIS_BAD_CHARACTERISTICS:                                    "An invalid characteristics table was used.",
	STATUS_NDIS_ADAPTER_NOT_FOUND:                                      "Failed to find the network interface or the network interface is not ready.",
	STATUS_NDIS_OPEN_FAILED:                                            "Failed to open the network interface.",
	STATUS_NDIS_DEVICE_FAILED:                                          "The network interface has encountered an internal unrecoverable failure.",
	STATUS_NDIS_MULTICAST_FULL:                                         "The multicast list on the network interface is full.",
	STATUS_NDIS_MULTICAST_EXISTS:                                       "An attempt was made to add a duplicate multicast address to the list.",
	STATUS_NDIS_MULTICAST_NOT_FOUND:                                    "At attempt was made to remove a multicast address that was never added.",
	STATUS_NDIS_REQUEST_ABORTED:                                        "The network interface aborted the request.",
	STATUS_NDIS_RESET_IN_PROGRESS:                                      "The network interface cannot process the request because it is being reset.",
	STATUS_NDIS_INVALID_PACKET:                                         "An attempt was made to send an invalid packet on a network interface.",
	STATUS_NDIS_INVALID_DEVICE_REQUEST:                                 "The specified request is not a valid operation for the target device.",
	STATUS_NDIS_ADAPTER_NOT_READY:                                      "The network interface is not ready to complete this operation.",
	STATUS_NDIS_INVALID_LENGTH:                                         "The length of the buffer submitted for this operation is not valid.",
	STATUS_NDIS_INVALID_DATA:                                           "The data used for this operation is not valid.",
	STATUS_NDIS_BUFFER_TOO_SHORT:                                       "The length of the submitted buffer for this operation is too small.",
	STATUS_NDIS_INVALID_OID:                                            "The network interface does not support this object identifier.",
	STATUS_NDIS_ADAPTER_REMOVED:                                        "The network interface has been removed.",
	STATUS_NDIS_UNSUPPORTED_MEDIA:                                      "The network interface does not support this media type.",
	STATUS_NDIS_GROUP_ADDRESS_IN_USE:                                   "An attempt was made to remove a token ring group address that is in use by other components.",
	STATUS_NDIS_FILE_NOT_FOUND:                                         "An attempt was made to map a file that cannot be found.",
	STATUS_NDIS_ERROR_READING_FILE:                                     "An error occurred while NDIS tried to map the file.",
	STATUS_NDIS_ALREADY_MAPPED:                                         "An attempt was made to map a file that is already mapped.",
	STATUS_NDIS_RESOURCE_CONFLICT:                                      "An attempt to allocate a hardware resource failed because the resource is used by another component.",
	STATUS_NDIS_MEDIA_DISCONNECTED:                                     "The I/O operation failed because the network media is disconnected or the wireless access point is out of range.",
	STATUS_NDIS_INVALID_ADDRESS:                                        "The network address used in the request is invalid.",
	STATUS_NDIS_PAUSED:                                                 "The offload operation on the network interface has been paused.",
	STATUS_NDIS_INTERFACE_NOT_FOUND:                                    "The network interface was not found.",
	STATUS_NDIS_UNSUPPORTED_REVISION:                                   "The revision number specified in the structure is not supported.",
	STATUS_NDIS_INVALID_PORT:                                           "The specified port does not exist on this network interface.",
	STATUS_NDIS_INVALID_PORT_STATE:                                     "The current state of the specified port on this network interface does not support the requested operation.",
	STATUS_NDIS_LOW_POWER_STATE:                                        "The miniport adapter is in a lower power state.",
	STATUS_NDIS_NOT_SUPPORTED:                                          "The network interface does not support this request.",
	STATUS_NDIS_OFFLOAD_POLICY:                                         "The TCP connection is not offloadable because of a local policy setting.",
	STATUS_NDIS_OFFLOAD_CONNECTION_REJECTED:                            "The TCP connection is not offloadable by the Chimney offload target.",
	STATUS_NDIS_OFFLOAD_PATH_REJECTED:                                  "The IP Path object is not in an offloadable state.",
	STATUS_NDIS_DOT11_AUTO_CONFIG_ENABLED:                              "The wireless LAN interface is in auto-configuration mode and does not support the requested parameter change operation.",
	STATUS_NDIS_DOT11_MEDIA_IN_USE:                                     "The wireless LAN interface is busy and cannot perform the requested operation.",
	STATUS_NDIS_DOT11_POWER_STATE_INVALID:                              "The wireless LAN interface is power down and does not support the requested operation.",
	STATUS_NDIS_PM_WOL_PATTERN_LIST_FULL:                               "The list of wake on LAN patterns is full.",
	STATUS_NDIS_PM_PROTOCOL_OFFLOAD_LIST_FULL:                          "The list of low power protocol offloads is full.",
	STATUS_IPSEC_BAD_SPI:                                               "The SPI in the packet does not match a valid IPsec SA.",
	STATUS_IPSEC_SA_LIFETIME_EXPIRED:                                   "The packet was received on an IPsec SA whose lifetime has expired.",
	STATUS_IPSEC_WRONG_SA:                                              "The packet was received on an IPsec SA that does not match the packet characteristics.",
	STATUS_IPSEC_REPLAY_CHECK_FAILED:                                   "The packet sequence number replay check failed.",
	STATUS_IPSEC_INVALID_PACKET:                                        "The IPsec header and/or trailer in the packet is invalid.",
	STATUS_IPSEC_INTEGRITY_CHECK_FAILED:                                "The IPsec integrity check failed.",
	STATUS_IPSEC_CLEAR_TEXT_DROP:                                       "IPsec dropped a clear text packet.",
	STATUS_IPSEC_AUTH_FIREWALL_DROP:                                    "IPsec dropped an incoming ESP packet in authenticated firewall mode.\u00a0 This drop is benign.",
	STATUS_IPSEC_THROTTLE_DROP:                                         "IPsec dropped a packet due to DOS throttle.",
	STATUS_IPSEC_DOSP_BLOCK:                                            "IPsec Dos Protection matched an explicit block rule.",
	STATUS_IPSEC_DOSP_RECEIVED_MULTICAST:                               "IPsec Dos Protection received an IPsec specific multicast packet which is not allowed.",
	STATUS_IPSEC_DOSP_INVALID_PACKET:                                   "IPsec Dos Protection received an incorrectly formatted packet.",
	STATUS_IPSEC_DOSP_STATE_LOOKUP_FAILED:                              "IPsec Dos Protection failed to lookup state.",
	STATUS_IPSEC_DOSP_MAX_ENTRIES:                                      "IPsec Dos Protection failed to create state because there are already maximum number of entries allowed by policy.",
	STATUS_IPSEC_DOSP_KEYMOD_NOT_ALLOWED:                               "IPsec Dos Protection received an IPsec negotiation packet for a keying module which is not allowed by policy.",
	STATUS_IPSEC_DOSP_MAX_PER_IP_RATELIMIT_QUEUES:                      "IPsec Dos Protection failed to create per internal IP ratelimit queue because there is already maximum number of queues allowed by policy.",
	STATUS_VOLMGR_MIRROR_NOT_SUPPORTED:                                 "The system does not support mirrored volumes.",
	STATUS_VOLMGR_RAID5_NOT_SUPPORTED:                                  "The system does not support RAID-5 volumes.",
	STATUS_VIRTDISK_PROVIDER_NOT_FOUND:                                 "A virtual disk support provider for the specified file was not found.",
	STATUS_VIRTDISK_NOT_VIRTUAL_DISK:                                   "The specified disk is not a virtual disk.",
	STATUS_VHD_PARENT_VHD_ACCESS_DENIED:                                "The chain of virtual hard disks is inaccessible. The process has not been granted access rights to the parent virtual hard disk for the differencing disk.",
	STATUS_VHD_CHILD_PARENT_SIZE_MISMATCH:                              "The chain of virtual hard disks is corrupted. There is a mismatch in the virtual sizes of the parent virtual hard disk and differencing disk.",
	STATUS_VHD_DIFFERENCING_CHAIN_CYCLE_DETECTED:                       "The chain of virtual hard disks is corrupted. A differencing disk is indicated in its own parent chain.",
	STATUS_VHD_DIFFERENCING_CHAIN_ERROR_IN_PARENT:                      "The chain of virtual hard disks is inaccessible. There was an error opening a virtual hard disk further up the chain.",
}
