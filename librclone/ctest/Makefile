CFLAGS = -g -Wall
LDFLAGS = -L. -lrclone -lpthread -ldl

static: ctest

shared:
	go build --buildmode=c-shared -o librclone.so github.com/rclone/rclone/librclone

ctest: ctest.o librclone.h
	$(CC) $(CFLAGS) -o $@ $< $(LDFLAGS)

ctest.o: ctest.c librclone.h
	$(CC) $(CFLAGS) -c $^ $(LDFLAGS)

build:
	go build

librclone.h:
	go build --buildmode=c-archive -o librclone.a github.com/rclone/rclone/librclone

clean:
	rm -f tmp ctest *.o *.a *.h *.gch
