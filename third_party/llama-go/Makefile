.PHONY: test clean

INCLUDE_PATH := $(abspath ./)
LIBRARY_PATH := $(abspath ./)

#
# Compile flags
#
BUILD_TYPE?=
BUILD_LINKAGE?=static
CFLAGS   = -I./llama.cpp -I. -O3 -DNDEBUG -std=c11 -fPIC -D_CRT_SECURE_NO_WARNINGS
CXXFLAGS = -I./llama.cpp -I. -I./llama.cpp/common -I./common -I./llama.cpp/ggml/include -I./llama.cpp/include -I./llama.cpp/vendor -O3 -DNDEBUG -std=c++17 -fPIC -D_CRT_SECURE_NO_WARNINGS
LDFLAGS  =

# warnings
CFLAGS   += -Wall -Wextra -Wpedantic -Wcast-qual -Wdouble-promotion -Wshadow -Wstrict-prototypes -Wpointer-arith -Wno-unused-function
CXXFLAGS += -Wall -Wextra -Wpedantic -Wcast-qual -Wno-unused-function

# Static linkage configurations
CMAKE_ARGS+=-DBUILD_SHARED_LIBS=OFF

COMMON_STATIC_LIBS = build/common/libllama-common.a build/common/libllama-common-base.a build/src/libllama.a build/ggml/src/ggml.a build/ggml/src/ggml-base.a build/ggml/src/ggml-cpu.a

llama.cpp/ggml-alloc.o: llama.cpp/ggml.o
	@mkdir -p llama.cpp
	@if [ -f build/ggml/src/CMakeFiles/ggml-base.dir/ggml-alloc.c.obj ]; then \
		cp -rf build/ggml/src/CMakeFiles/ggml-base.dir/ggml-alloc.c.obj llama.cpp/ggml-alloc.o; \
	else \
		cp -rf build/ggml/src/CMakeFiles/ggml-base.dir/ggml-alloc.c.o llama.cpp/ggml-alloc.o; \
	fi

llama.cpp/ggml.o:
	@mkdir -p build
	@cd build && cmake ../llama.cpp $(CMAKE_ARGS) -DLLAMA_CURL=OFF && cmake --build . --config Release --target ggml llama
	@mkdir -p llama.cpp
	@if [ -f build/ggml/src/CMakeFiles/ggml-base.dir/ggml.c.obj ]; then \
		cp -rf build/ggml/src/CMakeFiles/ggml-base.dir/ggml.c.obj llama.cpp/ggml.o; \
	else \
		cp -rf build/ggml/src/CMakeFiles/ggml-base.dir/ggml.c.o llama.cpp/ggml.o; \
	fi

llama.cpp/k_quants.o: llama.cpp/ggml.o
	@mkdir -p llama.cpp
	@if [ -f build/ggml/src/CMakeFiles/ggml-base.dir/ggml-quants.c.obj ]; then \
		cp -rf build/ggml/src/CMakeFiles/ggml-base.dir/ggml-quants.c.obj llama.cpp/k_quants.o; \
	fi

llama.cpp/llama.o: llama.cpp/ggml.o
	@mkdir -p llama.cpp
	@if [ -f build/src/CMakeFiles/llama.dir/llama.cpp.obj ]; then \
		cp -rf build/src/CMakeFiles/llama.dir/llama.cpp.obj llama.cpp/llama.o; \
	else \
		cp -rf build/src/CMakeFiles/llama.dir/llama.cpp.o llama.cpp/llama.o; \
	fi

llama.cpp/common.o: llama.cpp/ggml.o
	$(CXX) $(CXXFLAGS) -I./llama.cpp -I./llama.cpp/common -I./llama.cpp/ggml/include -I./llama.cpp/include llama.cpp/common/common.cpp -o llama.cpp/common.o -c $(LDFLAGS)

llama.cpp/sampling.o: llama.cpp/ggml.o
	$(CXX) $(CXXFLAGS) -I./llama.cpp -I./llama.cpp/common -I./llama.cpp/ggml/include -I./llama.cpp/include llama.cpp/common/sampling.cpp -o llama.cpp/sampling.o -c $(LDFLAGS)

llama.cpp/log.o: llama.cpp/ggml.o
	$(CXX) $(CXXFLAGS) -I./llama.cpp -I./llama.cpp/common -I./llama.cpp/ggml/include -I./llama.cpp/include llama.cpp/common/log.cpp -o llama.cpp/log.o -c $(LDFLAGS)

wrapper.o:
	$(CXX) $(CXXFLAGS) -I./llama.cpp -I./llama.cpp/common -I./llama.cpp/ggml/include -I./llama.cpp/include wrapper.cpp -o wrapper.o -c $(LDFLAGS)

.PHONY: vendor-headers

vendor-headers cgo_headers/.vendored:
	@echo "Vendoring llama.cpp headers..."
	@mkdir -p cgo_headers/llama.cpp/thirdparty
	@touch cgo_headers/.vendored

libbinding.a: cgo_headers/.vendored wrapper.o llama.cpp/ggml.o llama.cpp/ggml-alloc.o llama.cpp/k_quants.o llama.cpp/llama.o
	cd build && cmake --build . --target llama-common
	ar crs libbinding.a wrapper.o
	@echo "Copying static libraries..."
	cp $(COMMON_STATIC_LIBS) .
	@cp ggml-cpu.a libggml-cpu.a 2>/dev/null || true
	@cp ggml.a libggml.a 2>/dev/null || true
	@cp ggml-base.a libggml-base.a 2>/dev/null || true

clean:
	rm -rf *.o *.a *.so* build llama.cpp/*.o
