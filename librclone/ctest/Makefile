CFLAGS = -g -Wall

ifeq ($(OS),Windows_NT)
EXE = .exe
LIB = librclone.lib
LDFLAGS = -L. -lrclone -lwinmm -lws2_32 -lole32
else
EXE =
LIB = librclone.a
LDFLAGS = -L. -lrclone -lpthread -ldl
endif

ctest$(EXE): ctest.o $(LIB)
	$(CC) $(CFLAGS) -o $@ $< $(LDFLAGS)

ctest.o: ctest.c librclone.h
	$(CC) $(CFLAGS) -c $^ $(LDFLAGS)

$(LIB) librclone.h:
	go build --buildmode=c-archive -o $(LIB) github.com/rclone/rclone/librclone

test:	ctest$(EXE)
	./ctest$(EXE)

clean:
	rm -f tmp ctest ctest.exe *.o *.a *.lib *.h *.gch
