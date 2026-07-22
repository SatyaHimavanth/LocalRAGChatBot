# CMake generated Testfile for 
# Source directory: C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/llama.cpp/examples/eval-callback
# Build directory: C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/examples/eval-callback
# 
# This file includes the relevant testing commands required for 
# testing this directory and lists subdirectories to be tested as well.
add_test("test-eval-callback-download-model" "C:/msys64/ucrt64/bin/cmake.exe" "-DDEST=C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tinyllamas/stories15M-q4_0.gguf" "-DNAME=tinyllamas/stories15M-q4_0.gguf" "-DHASH=SHA256=66967fbece6dbe97886593fdbb73589584927e29119ec31f08090732d1861739" "-P" "C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/llama.cpp/cmake/download-models.cmake")
set_tests_properties("test-eval-callback-download-model" PROPERTIES  FIXTURES_SETUP "test-eval-callback-download-model" _BACKTRACE_TRIPLES "C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/llama.cpp/examples/eval-callback/CMakeLists.txt;17;add_test;C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/llama.cpp/examples/eval-callback/CMakeLists.txt;0;")
add_test("test-eval-callback" "C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/bin/llama-eval-callback.exe" "-m" "C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tinyllamas/stories15M-q4_0.gguf" "--prompt" "hello" "--seed" "42" "-ngl" "0")
set_tests_properties("test-eval-callback" PROPERTIES  FIXTURES_REQUIRED "test-eval-callback-download-model" _BACKTRACE_TRIPLES "C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/llama.cpp/examples/eval-callback/CMakeLists.txt;24;add_test;C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/llama.cpp/examples/eval-callback/CMakeLists.txt;0;")
