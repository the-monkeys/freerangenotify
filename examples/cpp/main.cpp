/**
 * FreeRangeNotify API — C++ Integration Example
 *
 * Prerequisites:
 *   - C++17 compiler (g++ 9+, clang 10+, MSVC 2019+)
 *   - libcurl development headers (apt install libcurl4-openssl-dev)
 *
 * Compile & Run:
 *   export FRN_API_KEY="frn_your_api_key_here"
 *   g++ -std=c++17 -o frn_example main.cpp -lcurl
 *   ./frn_example
 */

#include <cstdlib>
#include <fstream>
#include <iostream>
#include <sstream>
#include <string>

#include <curl/curl.h>

// ──────────────────────────────────────────────────────────────────────────────
// Configuration
// ──────────────────────────────────────────────────────────────────────────────

static const std::string BASE_URL = "https://freerangenotify.monkeys.support/v1";

static std::string getApiKey() {
    const char* key = std::getenv("FRN_API_KEY");
    if (!key || std::string(key).empty()) {
        std::cerr << "Error: FRN_API_KEY environment variable is not set.\n"
                  << "  export FRN_API_KEY=\"frn_your_api_key_here\"\n";
        std::exit(1);
    }
    return std::string(key);
}

// ──────────────────────────────────────────────────────────────────────────────
// libcurl helpers
// ──────────────────────────────────────────────────────────────────────────────

static size_t writeCallback(char* ptr, size_t size, size_t nmemb, void* userdata) {
    auto* response = static_cast<std::string*>(userdata);
    response->append(ptr, size * nmemb);
    return size * nmemb;
}

/**
 * Send an HTTP request and return the response body.
 *
 * @param method  HTTP method ("GET", "POST", "DELETE", etc.)
 * @param path    URL path appended to BASE_URL (e.g. "/notifications")
 * @param body    JSON body for POST/PUT (empty string for GET/DELETE)
 * @return        Response body as string
 */
static std::string httpRequest(const std::string& method,
                               const std::string& path,
                               const std::string& body = "") {
    CURL* curl = curl_easy_init();
    if (!curl) {
        std::cerr << "Failed to initialise libcurl\n";
        return "";
    }

    std::string url = BASE_URL + path;
    std::string response;

    // Headers
    struct curl_slist* headers = nullptr;
    headers = curl_slist_append(headers, ("X-API-Key: " + getApiKey()).c_str());
    headers = curl_slist_append(headers, "Content-Type: application/json");
    headers = curl_slist_append(headers, "Accept: application/json");

    curl_easy_setopt(curl, CURLOPT_URL, url.c_str());
    curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, writeCallback);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &response);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, 30L);

    if (method == "POST") {
        curl_easy_setopt(curl, CURLOPT_POST, 1L);
        curl_easy_setopt(curl, CURLOPT_POSTFIELDS, body.c_str());
    } else if (method == "DELETE") {
        curl_easy_setopt(curl, CURLOPT_CUSTOMREQUEST, "DELETE");
    } else if (method == "PUT") {
        curl_easy_setopt(curl, CURLOPT_CUSTOMREQUEST, "PUT");
        curl_easy_setopt(curl, CURLOPT_POSTFIELDS, body.c_str());
    }
    // GET is the default

    CURLcode res = curl_easy_perform(curl);
    if (res != CURLE_OK) {
        std::cerr << "curl error: " << curl_easy_strerror(res) << "\n";
    }

    long httpCode = 0;
    curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &httpCode);
    if (httpCode >= 400) {
        std::cerr << "HTTP " << httpCode << ": " << response << "\n";
    }

    curl_slist_free_all(headers);
    curl_easy_cleanup(curl);

    return response;
}

static void pretty(const std::string& label, const std::string& json) {
    std::cout << "\n=== " << label << " ===\n" << json << "\n\n";
}

// ──────────────────────────────────────────────────────────────────────────────
// 1. Send a Notification
// ──────────────────────────────────────────────────────────────────────────────

static void sendNotification() {
    std::cout << "──── Send Notification ────\n";

    std::string body = R"({
        "user_id": "user-uuid-or-external-id",
        "channel": "email",
        "priority": "normal",
        "template_id": "welcome-email",
        "data": {
            "name": "Jane Doe",
            "company": "Acme Corp"
        }
    })";

    auto resp = httpRequest("POST", "/notifications", body);
    pretty("Send Notification Response", resp);
}

// ──────────────────────────────────────────────────────────────────────────────
// 2. Send Bulk Notifications
// ──────────────────────────────────────────────────────────────────────────────

static void sendBulkNotifications() {
    std::cout << "──── Send Bulk Notifications ────\n";

    std::string body = R"({
        "user_ids": ["user-1-uuid", "user-2-uuid", "user-3-uuid"],
        "channel": "push",
        "priority": "high",
        "template_id": "flash-sale",
        "data": {
            "discount": "25%",
            "expires": "2026-06-10T00:00:00Z"
        }
    })";

    auto resp = httpRequest("POST", "/notifications/bulk", body);
    pretty("Bulk Send Response", resp);
}

// ──────────────────────────────────────────────────────────────────────────────
// 3. List Notifications
// ──────────────────────────────────────────────────────────────────────────────

static void listNotifications() {
    std::cout << "──── List Notifications ────\n";

    auto resp = httpRequest("GET", "/notifications?page=1&page_size=10");
    pretty("List Notifications Response", resp);
}

// ──────────────────────────────────────────────────────────────────────────────
// 4. Send OTP
// ──────────────────────────────────────────────────────────────────────────────

static void sendOTP() {
    std::cout << "──── Send OTP ────\n";

    std::string body = R"({
        "channel": "sms",
        "recipient": "+14155551234"
    })";

    auto resp = httpRequest("POST", "/otp/send", body);
    pretty("Send OTP Response", resp);
}

// ──────────────────────────────────────────────────────────────────────────────
// 5. Verify OTP
// ──────────────────────────────────────────────────────────────────────────────

static void verifyOTP(const std::string& requestId, const std::string& code) {
    std::cout << "──── Verify OTP ────\n";

    std::string body = R"({"request_id": ")" + requestId + R"(", "code": ")" + code + R"("})";

    auto resp = httpRequest("POST", "/otp/verify", body);
    pretty("Verify OTP Response", resp);
}

// ──────────────────────────────────────────────────────────────────────────────
// 6. Upload a File (Invoice)
// ──────────────────────────────────────────────────────────────────────────────

static void uploadFile(const std::string& filePath) {
    std::cout << "──── Upload File ────\n";

    CURL* curl = curl_easy_init();
    if (!curl) {
        std::cerr << "Failed to initialise libcurl\n";
        return;
    }

    // Extract filename from path
    std::string filename = filePath;
    auto pos = filePath.find_last_of("/\\");
    if (pos != std::string::npos) {
        filename = filePath.substr(pos + 1);
    }

    // Build multipart form
    curl_mime* mime = curl_mime_init(curl);
    curl_mimepart* part = curl_mime_addpart(mime);
    curl_mime_name(part, "file");
    curl_mime_filedata(part, filePath.c_str());
    curl_mime_filename(part, filename.c_str());

    // Headers
    struct curl_slist* headers = nullptr;
    headers = curl_slist_append(headers, ("X-API-Key: " + getApiKey()).c_str());
    headers = curl_slist_append(headers, "Accept: application/json");

    std::string response;

    curl_easy_setopt(curl, CURLOPT_URL, (BASE_URL + "/files").c_str());
    curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
    curl_easy_setopt(curl, CURLOPT_MIMEPOST, mime);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, writeCallback);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &response);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, 60L);

    CURLcode res = curl_easy_perform(curl);
    if (res != CURLE_OK) {
        std::cerr << "curl error: " << curl_easy_strerror(res) << "\n";
    }

    pretty("Upload File Response", response);

    curl_mime_free(mime);
    curl_slist_free_all(headers);
    curl_easy_cleanup(curl);
}

// ──────────────────────────────────────────────────────────────────────────────
// 7. Quick Send
// ──────────────────────────────────────────────────────────────────────────────

static void quickSend() {
    std::cout << "──── Quick Send ────\n";

    std::string body = R"({
        "to": "jane@example.com",
        "channel": "email",
        "subject": "Your Invoice is Ready",
        "body": "Hi Jane, your invoice #1234 is attached.",
        "priority": "normal"
    })";

    auto resp = httpRequest("POST", "/quick-send", body);
    pretty("Quick Send Response", resp);
}

// ──────────────────────────────────────────────────────────────────────────────
// Main
// ──────────────────────────────────────────────────────────────────────────────

int main() {
    curl_global_init(CURL_GLOBAL_DEFAULT);

    std::cout << "╔══════════════════════════════════════════════════════════╗\n"
              << "║   FreeRangeNotify — C++ Integration Examples            ║\n"
              << "╚══════════════════════════════════════════════════════════╝\n\n";

    // 1. Send a single notification
    sendNotification();

    // 2. Send bulk notifications
    sendBulkNotifications();

    // 3. List recent notifications
    listNotifications();

    // 4. Send an OTP via SMS
    sendOTP();

    // 5. Verify an OTP (replace with real values)
    verifyOTP("req_placeholder_id", "123456");

    // 6. Upload a file (uncomment and provide a real path)
    // uploadFile("/path/to/invoice.pdf");

    // 7. Quick send
    quickSend();

    curl_global_cleanup();
    return 0;
}
