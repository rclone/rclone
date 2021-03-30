/*
This is a very simple test/demo program for librclone's C interface
*/
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <dlfcn.h>
#include "librclone.h"

void testRPC(char *method, char *in) {
    struct RcloneRPCResult out = RcloneRPC(method, in);
    printf("status: %d\n", out.Status);
    printf("output: %s\n", out.Output);
    free(out.Output);
}

// noop command
void testNoOp() {
    printf("test rc/noop\n");
    struct RcloneRPCResult out = RcloneRPC("rc/noop", "{"
            " \"p1\": [1,\"2\",null,4],"
            " \"p2\": { \"a\":1, \"b\":2 } "
            "}");
    printf("status: %d\n", out.Status);
    printf("output: %s\n", out.Output);
    const char *expected =
        "{\n"
        "\t\"p1\": [\n"
        "\t\t1,\n"
        "\t\t\"2\",\n"
        "\t\tnull,\n"
        "\t\t4\n"
        "\t],\n"
        "\t\"p2\": {\n"
        "\t\t\"a\": 1,\n"
        "\t\t\"b\": 2\n"
        "\t}\n"
        "}\n";
    if (strcmp(expected, out.Output) != 0) {
        fprintf(stderr, "Wrong output.\nWant:\n%s\nGot:\n%s\n", expected, out.Output);
        exit(EXIT_FAILURE);
    }
    if (out.Status != 200) {
        fprintf(stderr, "Wrong status: want: %d: got: %d\n", 200, out.Status);
        exit(EXIT_FAILURE);
    }
    free(out.Output);
}

// error command
void testError() {
    printf("test rc/error\n");
    struct RcloneRPCResult out = RcloneRPC("rc/error",
            "{"
            " \"p1\": [1,\"2\",null,4],"
            " \"p2\": { \"a\":1, \"b\":2 } "
            "}");
    printf("status: %d\n", out.Status);
    printf("output: %s\n", out.Output);
    const char *expected =
        "{\n"
        "\t\"error\": \"arbitrary error on input map[p1:[1 2 \\u003cnil\\u003e 4] p2:map[a:1 b:2]]\",\n"
        "\t\"input\": {\n"
        "\t\t\"p1\": [\n"
        "\t\t\t1,\n"
        "\t\t\t\"2\",\n"
        "\t\t\tnull,\n"
        "\t\t\t4\n"
        "\t\t],\n"
        "\t\t\"p2\": {\n"
        "\t\t\t\"a\": 1,\n"
        "\t\t\t\"b\": 2\n"
        "\t\t}\n"
        "\t},\n"
        "\t\"path\": \"rc/error\",\n"
        "\t\"status\": 500\n"
        "}\n";
    if (strcmp(expected, out.Output) != 0) {
        fprintf(stderr, "Wrong output.\nWant:\n%s\nGot:\n%s\n", expected, out.Output);
        exit(EXIT_FAILURE);
    }
    if (out.Status != 500) {
        fprintf(stderr, "Wrong status: want: %d: got: %d\n", 500, out.Status);
        exit(EXIT_FAILURE);
    }
    free(out.Output);
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
    /* testCopyFile(); */
    /* testListRemotes(); */

    RcloneFinalize();
    return EXIT_SUCCESS;
}
