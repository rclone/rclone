#include <stdio.h>
#include <string.h>
#include "smc_darwin.h"

#define IOSERVICE_SMC "AppleSMC"
#define IOSERVICE_MODEL "IOPlatformExpertDevice"

#define DATA_TYPE_SP78 "sp78"

typedef enum {
  kSMCUserClientOpen = 0,
  kSMCUserClientClose = 1,
  kSMCHandleYPCEvent = 2,
  kSMCReadKey = 5,
  kSMCWriteKey = 6,
  kSMCGetKeyCount = 7,
  kSMCGetKeyFromIndex = 8,
  kSMCGetKeyInfo = 9,
} selector_t;

typedef struct {
  unsigned char major;
  unsigned char minor;
  unsigned char build;
  unsigned char reserved;
  unsigned short release;
} SMCVersion;

typedef struct {
  uint16_t version;
  uint16_t length;
  uint32_t cpuPLimit;
  uint32_t gpuPLimit;
  uint32_t memPLimit;
} SMCPLimitData;

typedef struct {
  IOByteCount data_size;
  uint32_t data_type;
  uint8_t data_attributes;
} SMCKeyInfoData;

typedef struct {
  uint32_t key;
  SMCVersion vers;
  SMCPLimitData p_limit_data;
  SMCKeyInfoData key_info;
  uint8_t result;
  uint8_t status;
  uint8_t data8;
  uint32_t data32;
  uint8_t bytes[32];
} SMCParamStruct;

typedef enum {
  kSMCSuccess = 0,
  kSMCError = 1,
  kSMCKeyNotFound = 0x84,
} kSMC_t;

typedef struct {
  uint8_t data[32];
  uint32_t data_type;
  uint32_t data_size;
  kSMC_t kSMC;
} smc_return_t;

static const int SMC_KEY_SIZE = 4; // number of characters in an SMC key.
static io_connect_t conn;          // our connection to the SMC.

kern_return_t gopsutil_v3_open_smc(void) {
  kern_return_t result;
  io_service_t service;

  service = IOServiceGetMatchingService(0, IOServiceMatching(IOSERVICE_SMC));
  if (service == 0) {
    // Note: IOServiceMatching documents 0 on failure
    printf("ERROR: %s NOT FOUND\n", IOSERVICE_SMC);
    return kIOReturnError;
  }

  result = IOServiceOpen(service, mach_task_self(), 0, &conn);
  IOObjectRelease(service);

  return result;
}

kern_return_t gopsutil_v3_close_smc(void) { return IOServiceClose(conn); }

static uint32_t to_uint32(char *key) {
  uint32_t ans = 0;
  uint32_t shift = 24;

  if (strlen(key) != SMC_KEY_SIZE) {
    return 0;
  }

  for (int i = 0; i < SMC_KEY_SIZE; i++) {
    ans += key[i] << shift;
    shift -= 8;
  }

  return ans;
}

static kern_return_t call_smc(SMCParamStruct *input, SMCParamStruct *output) {
  kern_return_t result;
  size_t input_cnt = sizeof(SMCParamStruct);
  size_t output_cnt = sizeof(SMCParamStruct);

  result = IOConnectCallStructMethod(conn, kSMCHandleYPCEvent, input, input_cnt,
                                     output, &output_cnt);

  if (result != kIOReturnSuccess) {
    result = err_get_code(result);
  }
  return result;
}

static kern_return_t read_smc(char *key, smc_return_t *result_smc) {
  kern_return_t result;
  SMCParamStruct input;
  SMCParamStruct output;

  memset(&input, 0, sizeof(SMCParamStruct));
  memset(&output, 0, sizeof(SMCParamStruct));
  memset(result_smc, 0, sizeof(smc_return_t));

  input.key = to_uint32(key);
  input.data8 = kSMCGetKeyInfo;

  result = call_smc(&input, &output);
  result_smc->kSMC = output.result;

  if (result != kIOReturnSuccess || output.result != kSMCSuccess) {
    return result;
  }

  result_smc->data_size = output.key_info.data_size;
  result_smc->data_type = output.key_info.data_type;

  input.key_info.data_size = output.key_info.data_size;
  input.data8 = kSMCReadKey;

  result = call_smc(&input, &output);
  result_smc->kSMC = output.result;

  if (result != kIOReturnSuccess || output.result != kSMCSuccess) {
    return result;
  }

  memcpy(result_smc->data, output.bytes, sizeof(output.bytes));

  return result;
}

double gopsutil_v3_get_temperature(char *key) {
  kern_return_t result;
  smc_return_t result_smc;

  result = read_smc(key, &result_smc);

  if (!(result == kIOReturnSuccess) && result_smc.data_size == 2 &&
      result_smc.data_type == to_uint32(DATA_TYPE_SP78)) {
    return 0.0;
  }

  return (double)result_smc.data[0];
}
