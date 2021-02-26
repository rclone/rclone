#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <dlfcn.h>
#include "librclone.h"

void testRPC(char *method, char *in) {
    struct RcloneRPC_return out = RcloneRPC(method, in);
    printf("status: %d\n", out.r1);
    printf("output: %s\n", out.r0);
    free(out.r0);
}

// noop command
void testNoOp() {
    printf("test rc/noop\n");
    testRPC("rc/noop",
            "{"
            " \"p1\": [1,\"2\",null,4],"
            " \"p2\": { \"a\":1, \"b\":2 } "
            "}");
}

// error command
void testError() {
    printf("test rc/error\n");
    testRPC("rc/error",
            "{"
            " \"p1\": [1,\"2\",null,4],"
            " \"p2\": { \"a\":1, \"b\":2 } "
            "}");
}

// copy file using "operations/copyfile" command
void testCopyFile() {
    printf("test operations/copyfile\n");
    testRPC("operations/copyfile",
            "{"
                "\"srcFs\": \"/tmp\","
                "\"srcRemote\": \"tmpfile\","
                "\"dstFs\": \"/tmp\","
                "\"dstRemote\": \"tmpfile2\""
            "}");
}

// list the remotes
void testListRemotes() {
    printf("test operations/listremotes\n");
    testRPC("config/listremotes", "{}");
}

int main(int argc, char** argv) {
    printf("c main begin\n");
    RcloneInitialize();

    testNoOp();
    testError();
    testCopyFile();
    testListRemotes();

    RcloneFinalize();
    return EXIT_SUCCESS;
}
