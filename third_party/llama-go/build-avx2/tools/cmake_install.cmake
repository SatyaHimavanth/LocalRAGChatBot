# Install script for directory: C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/llama.cpp/tools

# Set the install prefix
if(NOT DEFINED CMAKE_INSTALL_PREFIX)
  set(CMAKE_INSTALL_PREFIX "C:/Program Files (x86)/llama.cpp")
endif()
string(REGEX REPLACE "/$" "" CMAKE_INSTALL_PREFIX "${CMAKE_INSTALL_PREFIX}")

# Set the install configuration name.
if(NOT DEFINED CMAKE_INSTALL_CONFIG_NAME)
  if(BUILD_TYPE)
    string(REGEX REPLACE "^[^A-Za-z0-9_]+" ""
           CMAKE_INSTALL_CONFIG_NAME "${BUILD_TYPE}")
  else()
    set(CMAKE_INSTALL_CONFIG_NAME "Release")
  endif()
  message(STATUS "Install configuration: \"${CMAKE_INSTALL_CONFIG_NAME}\"")
endif()

# Set the component getting installed.
if(NOT CMAKE_INSTALL_COMPONENT)
  if(COMPONENT)
    message(STATUS "Install component: \"${COMPONENT}\"")
    set(CMAKE_INSTALL_COMPONENT "${COMPONENT}")
  else()
    set(CMAKE_INSTALL_COMPONENT)
  endif()
endif()

# Is this installation the result of a crosscompile?
if(NOT DEFINED CMAKE_CROSSCOMPILING)
  set(CMAKE_CROSSCOMPILING "FALSE")
endif()

# Set path to fallback-tool for dependency-resolution.
if(NOT DEFINED CMAKE_OBJDUMP)
  set(CMAKE_OBJDUMP "C:/msys64/ucrt64/bin/objdump.exe")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/batched-bench/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/gguf-split/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/imatrix/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/llama-bench/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/completion/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/perplexity/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/quantize/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/ui/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/cli/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/server/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/tokenize/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/parser/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/tts/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/mtmd/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/cvector-generator/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/export-lora/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/fit-params/cmake_install.cmake")
endif()

if(NOT CMAKE_INSTALL_LOCAL_ONLY)
  # Include the install script for the subdirectory.
  include("C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/results/cmake_install.cmake")
endif()

string(REPLACE ";" "\n" CMAKE_INSTALL_MANIFEST_CONTENT
       "${CMAKE_INSTALL_MANIFEST_FILES}")
if(CMAKE_INSTALL_LOCAL_ONLY)
  file(WRITE "C:/Users/hp/Desktop/Projects/LocalRAGChatBot/third_party/llama-go/build-avx2/tools/install_local_manifest.txt"
     "${CMAKE_INSTALL_MANIFEST_CONTENT}")
endif()
