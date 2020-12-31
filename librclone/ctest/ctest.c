#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <dlfcn.h>
#include "librclone.h"

// copy file using "operations/copyfile" command
void testCopyFile() {
    struct CRPC_return res = CRPC("operations/copyfile", "{ \"srcFs\": \"/tmp\", \"srcRemote\": \"tmpfile\", \"dstFs\": \"/tmp\", \"dstRemote\": \"tmpfile2\" }");
    printf("%d\n", res.r1); // status
    printf("%s\n", res.r0); // output
    free(res.r0);
}

// noop command
void testNoOp() {
    struct CRPC_return res = CRPC("rc/noop", "{ \"p1\": [1,\"2\",null,4], \"p2\": { \"a\":1, \"b\":2 } }");
    printf("%d\n", res.r1); // status
    printf("%s\n", res.r0); // output
    free(res.r0);
}

int main(int argc, char** argv) {
    printf("c main begin\n");
    Cinit();

    //testNoOp();
    testCopyFile();

    Cdestroy();
    return EXIT_SUCCESS;
}
