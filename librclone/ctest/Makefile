CFLAGS = -g -Wall
LDFLAGS = -L. -lrclone -lpthread -ldl

ctest: ctest.o librclone.a
	$(CC) $(CFLAGS) -o $@ $< $(LDFLAGS)

ctest.o: ctest.c librclone.h
	$(CC) $(CFLAGS) -c $^ $(LDFLAGS)

librclone.a librclone.h:
	go build --buildmode=c-archive -o librclone.a github.com/rclone/rclone/librclone

test:	ctest
	./ctest

clean:
	rm -f tmp ctest *.o *.a *.h *.gch
