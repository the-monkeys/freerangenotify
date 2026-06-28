/**
 * FreeRangeNotify API — Java Integration Example
 *
 * Prerequisites:
 *   - Java 11+ (uses java.net.http.HttpClient)
 *
 * Compile & Run:
 *   export FRN_API_KEY="frn_your_api_key_here"
 *   javac FreeRangeNotifyExample.java
 *   java FreeRangeNotifyExample
 */

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.UUID;

public class FreeRangeNotifyExample {

    // ── Configuration ──────────────────────────────────────────────────────

    private static final String BASE_URL = "https://freerangenotify.monkeys.support/v1";

    private static final HttpClient client = HttpClient.newBuilder()
            .connectTimeout(Duration.ofSeconds(10))
            .build();

    private static String getApiKey() {
        String key = System.getenv("FRN_API_KEY");
        if (key == null || key.isBlank()) {
            System.err.println("Error: FRN_API_KEY environment variable is not set.");
            System.err.println("  export FRN_API_KEY=\"frn_your_api_key_here\"");
            System.exit(1);
        }
        return key;
    }

    // ── HTTP helpers ───────────────────────────────────────────────────────

    private static String sendJson(String method, String path, String jsonBody)
            throws IOException, InterruptedException {

        HttpRequest.Builder builder = HttpRequest.newBuilder()
                .uri(URI.create(BASE_URL + path))
                .header("X-API-Key", getApiKey())
                .header("Content-Type", "application/json")
                .header("Accept", "application/json")
                .timeout(Duration.ofSeconds(30));

        switch (method.toUpperCase()) {
            case "GET":
                builder.GET();
                break;
            case "POST":
                builder.POST(HttpRequest.BodyPublishers.ofString(jsonBody != null ? jsonBody : ""));
                break;
            case "DELETE":
                builder.DELETE();
                break;
            default:
                builder.method(method.toUpperCase(),
                        HttpRequest.BodyPublishers.ofString(jsonBody != null ? jsonBody : ""));
        }

        HttpResponse<String> response = client.send(builder.build(),
                HttpResponse.BodyHandlers.ofString());

        if (response.statusCode() >= 400) {
            System.err.printf("HTTP %d: %s%n", response.statusCode(), response.body());
        }

        return response.body();
    }

    private static void pretty(String label, String json) {
        System.out.printf("%n=== %s ===%n%s%n%n", label, json);
    }

    // ── 1. Send a Notification ─────────────────────────────────────────────

    private static void sendNotification() throws IOException, InterruptedException {
        System.out.println("──── Send Notification ────");

        String json = """
                {
                    "user_id": "user-uuid-or-external-id",
                    "channel": "email",
                    "priority": "normal",
                    "template_id": "welcome-email",
                    "data": {
                        "name": "Jane Doe",
                        "company": "Acme Corp"
                    }
                }
                """;

        String resp = sendJson("POST", "/notifications", json);
        pretty("Send Notification Response", resp);
    }

    // ── 2. Send Bulk Notifications ──────────────────────────────────────────

    private static void sendBulkNotifications() throws IOException, InterruptedException {
        System.out.println("──── Send Bulk Notifications ────");

        String json = """
                {
                    "user_ids": ["user-1-uuid", "user-2-uuid", "user-3-uuid"],
                    "channel": "push",
                    "priority": "high",
                    "template_id": "flash-sale",
                    "data": {
                        "discount": "25%",
                        "expires": "2026-06-10T00:00:00Z"
                    }
                }
                """;

        String resp = sendJson("POST", "/notifications/bulk", json);
        pretty("Bulk Send Response", resp);
    }

    // ── 3. List Notifications ───────────────────────────────────────────────

    private static void listNotifications() throws IOException, InterruptedException {
        System.out.println("──── List Notifications ────");

        String resp = sendJson("GET", "/notifications?page=1&page_size=10", null);
        pretty("List Notifications Response", resp);
    }

    // ── 4. Send OTP ────────────────────────────────────────────────────────

    private static void sendOTP() throws IOException, InterruptedException {
        System.out.println("──── Send OTP ────");

        String json = """
                {
                    "channel": "sms",
                    "recipient": "+14155551234"
                }
                """;

        String resp = sendJson("POST", "/otp/send", json);
        pretty("Send OTP Response", resp);
    }

    // ── 5. Verify OTP ──────────────────────────────────────────────────────

    private static void verifyOTP(String requestId, String code)
            throws IOException, InterruptedException {
        System.out.println("──── Verify OTP ────");

        String json = String.format("""
                {
                    "request_id": "%s",
                    "code": "%s"
                }
                """, requestId, code);

        String resp = sendJson("POST", "/otp/verify", json);
        pretty("Verify OTP Response", resp);
    }

    // ── 6. Upload a File (Invoice) ─────────────────────────────────────────

    private static void uploadFile(String filePath)
            throws IOException, InterruptedException {
        System.out.println("──── Upload File ────");

        Path path = Path.of(filePath);
        if (!Files.exists(path)) {
            System.err.println("File not found: " + filePath);
            return;
        }

        String boundary = UUID.randomUUID().toString();
        byte[] fileBytes = Files.readAllBytes(path);
        String fileName = path.getFileName().toString();
        String mimeType = Files.probeContentType(path);
        if (mimeType == null) mimeType = "application/octet-stream";

        // Build multipart body manually
        String header = "--" + boundary + "\r\n"
                + "Content-Disposition: form-data; name=\"file\"; filename=\"" + fileName + "\"\r\n"
                + "Content-Type: " + mimeType + "\r\n\r\n";
        String footer = "\r\n--" + boundary + "--\r\n";

        byte[] headerBytes = header.getBytes();
        byte[] footerBytes = footer.getBytes();
        byte[] body = new byte[headerBytes.length + fileBytes.length + footerBytes.length];
        System.arraycopy(headerBytes, 0, body, 0, headerBytes.length);
        System.arraycopy(fileBytes, 0, body, headerBytes.length, fileBytes.length);
        System.arraycopy(footerBytes, 0, body, headerBytes.length + fileBytes.length, footerBytes.length);

        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(BASE_URL + "/files"))
                .header("X-API-Key", getApiKey())
                .header("Content-Type", "multipart/form-data; boundary=" + boundary)
                .header("Accept", "application/json")
                .POST(HttpRequest.BodyPublishers.ofByteArray(body))
                .build();

        HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());
        pretty("Upload File Response", response.body());
    }

    // ── 7. Quick Send ──────────────────────────────────────────────────────

    private static void quickSend() throws IOException, InterruptedException {
        System.out.println("──── Quick Send ────");

        String json = """
                {
                    "to": "jane@example.com",
                    "channel": "email",
                    "subject": "Your Invoice is Ready",
                    "body": "Hi Jane, your invoice #1234 is attached.",
                    "priority": "normal"
                }
                """;

        String resp = sendJson("POST", "/quick-send", json);
        pretty("Quick Send Response", resp);
    }

    // ── Main ────────────────────────────────────────────────────────────────

    public static void main(String[] args) throws IOException, InterruptedException {
        System.out.println("╔══════════════════════════════════════════════════════════╗");
        System.out.println("║   FreeRangeNotify — Java Integration Examples           ║");
        System.out.println("╚══════════════════════════════════════════════════════════╝");

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
    }
}
