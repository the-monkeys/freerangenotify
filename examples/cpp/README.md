# FreeRangeNotify — C++ Example

## Prerequisites

- C++17 compiler (g++ 9+, clang 10+, or MSVC 2019+)
- [libcurl](https://curl.se/libcurl/) development headers

### Install libcurl

**Ubuntu/Debian:**
```bash
sudo apt install libcurl4-openssl-dev
```

**macOS (Homebrew):**
```bash
brew install curl
```

**Windows (vcpkg):**
```bash
vcpkg install curl
```

## Compile & Run

```bash
export FRN_API_KEY="frn_your_api_key_here"
cd examples/cpp
g++ -std=c++17 -o frn_example main.cpp -lcurl
./frn_example
```

### CMake (alternative)

```cmake
cmake_minimum_required(VERSION 3.14)
project(frn_example)

set(CMAKE_CXX_STANDARD 17)
find_package(CURL REQUIRED)

add_executable(frn_example main.cpp)
target_link_libraries(frn_example CURL::libcurl)
```

## What's Covered

| Function | Description |
|----------|-------------|
| `sendNotification()` | Send a single notification via email |
| `sendBulkNotifications()` | Send push notifications to multiple users |
| `listNotifications()` | Paginated list of recent notifications |
| `sendOTP()` | Send a one-time passcode via SMS |
| `verifyOTP()` | Verify a received OTP code |
| `uploadFile()` | Upload an invoice/file via multipart form (curl_mime) |
| `quickSend()` | Simplified send using email or external ID |

## Notes

- Uses raw JSON string literals (`R"(...)"`) — no JSON library needed for these examples.
- For production use, consider a JSON library like [nlohmann/json](https://github.com/nlohmann/json) for safe serialization.
- File upload uses `curl_mime` API (libcurl 7.56+).
