.PHONY: libs

CFLAGS=$(OSFLAGS) -Wall -m64 -O3 -mtune=native -fPIC

libs:
	cd libs; \
	$(CC) $(CFLAGS) -c -o shabal64.o shabal64.s; \
	$(CC) $(CFLAGS) -c -o mshabal_sse4.o mshabal_sse4.c; \
	$(CC) $(CFLAGS) -mavx2 -c -o mshabal256_avx2.o mshabal256_avx2.c; \
	$(CC) $(CFLAGS) -shared -o libburstmath.a burstmath.c shabal64.o mshabal_sse4.o mshabal256_avx2.o -lpthread -std=gnu99;
cover:
	go test -coverprofile=cover.out
	go tool cover -html=cover.out
