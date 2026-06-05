/**
 * FreeRangeNotify API — JavaScript / Node.js Integration Example
 *
 * Prerequisites:
 *   - Node.js 18+ (uses native fetch, no external dependencies)
 *
 * Usage:
 *   export FRN_API_KEY="frn_your_api_key_here"
 *   node main.mjs
 */

import { readFile } from "node:fs/promises";
import { basename } from "node:path";

// ──────────────────────────────────────────────────────────────────────────────
// Configuration
// ──────────────────────────────────────────────────────────────────────────────

const BASE_URL = "https://freerangenotify.monkeys.support/v1";

function getApiKey() {
    const key = process.env.FRN_API_KEY;
    if (!key) {
        console.error("Error: FRN_API_KEY environment variable is not set.");
        console.error('  export FRN_API_KEY="frn_your_api_key_here"');
        process.exit(1);
    }
    return key;
}

// ──────────────────────────────────────────────────────────────────────────────
// HTTP helpers
// ──────────────────────────────────────────────────────────────────────────────

async function apiRequest(method, path, body = null) {
    const url = `${BASE_URL}${path}`;
    const options = {
        method,
        headers: {
            "X-API-Key": getApiKey(),
            Accept: "application/json",
        },
    };

    if (body !== null) {
        options.headers["Content-Type"] = "application/json";
        options.body = JSON.stringify(body);
    }

    const resp = await fetch(url, options);
    const data = await resp.json().catch(() => resp.text());

    if (!resp.ok) {
        console.error(`HTTP ${resp.status}:`, data);
    }

    return { status: resp.status, data };
}

function pretty(label, data) {
    console.log(`\n=== ${label} ===`);
    console.log(JSON.stringify(data, null, 2));
    console.log();
}

// ──────────────────────────────────────────────────────────────────────────────
// 1. Send a Notification
// ──────────────────────────────────────────────────────────────────────────────

async function sendNotification() {
    console.log("──── Send Notification ────");

    const { data } = await apiRequest("POST", "/notifications", {
        user_id: "user-uuid-or-external-id",
        channel: "email",
        priority: "normal",
        template_id: "welcome-email",
        data: {
            name: "Jane Doe",
            company: "Acme Corp",
        },
    });

    pretty("Send Notification Response", data);
    return data;
}

// ──────────────────────────────────────────────────────────────────────────────
// 2. Send Bulk Notifications
// ──────────────────────────────────────────────────────────────────────────────

async function sendBulkNotifications() {
    console.log("──── Send Bulk Notifications ────");

    const { data } = await apiRequest("POST", "/notifications/bulk", {
        user_ids: ["user-1-uuid", "user-2-uuid", "user-3-uuid"],
        channel: "push",
        priority: "high",
        template_id: "flash-sale",
        data: {
            discount: "25%",
            expires: "2026-06-10T00:00:00Z",
        },
    });

    pretty("Bulk Send Response", data);
    return data;
}

// ──────────────────────────────────────────────────────────────────────────────
// 3. List Notifications
// ──────────────────────────────────────────────────────────────────────────────

async function listNotifications(page = 1, pageSize = 10) {
    console.log("──── List Notifications ────");

    const { data } = await apiRequest(
        "GET",
        `/notifications?page=${page}&page_size=${pageSize}`
    );

    pretty("List Notifications Response", data);
    return data;
}

// ──────────────────────────────────────────────────────────────────────────────
// 4. Send OTP
// ──────────────────────────────────────────────────────────────────────────────

async function sendOTP(channel = "sms", recipient = "+14155551234") {
    console.log("──── Send OTP ────");

    const { data } = await apiRequest("POST", "/otp/send", {
        channel,
        recipient,
        // Optional customisations:
        // length: 6,
        // ttl_seconds: 300,
        // max_attempts: 5,
    });

    pretty("Send OTP Response", data);
    return data;
}

// ──────────────────────────────────────────────────────────────────────────────
// 5. Verify OTP
// ──────────────────────────────────────────────────────────────────────────────

async function verifyOTP(requestId, code) {
    console.log("──── Verify OTP ────");

    const { data } = await apiRequest("POST", "/otp/verify", {
        request_id: requestId,
        code,
    });

    pretty("Verify OTP Response", data);
    return data;
}

// ──────────────────────────────────────────────────────────────────────────────
// 6. Upload a File (Invoice)
// ──────────────────────────────────────────────────────────────────────────────

async function uploadFile(filePath) {
    console.log("──── Upload File ────");

    const fileBuffer = await readFile(filePath);
    const blob = new Blob([fileBuffer]);
    const formData = new FormData();
    formData.append("file", blob, basename(filePath));

    const resp = await fetch(`${BASE_URL}/files`, {
        method: "POST",
        headers: {
            "X-API-Key": getApiKey(),
            Accept: "application/json",
        },
        body: formData,
    });

    const data = await resp.json().catch(() => resp.text());
    pretty("Upload File Response", data);
    return data;
}

// ──────────────────────────────────────────────────────────────────────────────
// 7. List Files
// ──────────────────────────────────────────────────────────────────────────────

async function listFiles() {
    console.log("──── List Files ────");

    const { data } = await apiRequest("GET", "/files");
    pretty("List Files Response", data);
    return data;
}

// ──────────────────────────────────────────────────────────────────────────────
// 8. Quick Send
// ──────────────────────────────────────────────────────────────────────────────

async function quickSend() {
    console.log("──── Quick Send ────");

    const { data } = await apiRequest("POST", "/quick-send", {
        to: "jane@example.com",
        channel: "email",
        subject: "Your Invoice is Ready",
        body: "Hi Jane, your invoice #1234 is attached.",
        priority: "normal",
    });

    pretty("Quick Send Response", data);
    return data;
}

// ──────────────────────────────────────────────────────────────────────────────
// Main
// ──────────────────────────────────────────────────────────────────────────────

async function main() {
    console.log("╔══════════════════════════════════════════════════════════╗");
    console.log("║   FreeRangeNotify — JavaScript Integration Examples     ║");
    console.log("╚══════════════════════════════════════════════════════════╝");

    // 1. Send a single notification
    await sendNotification();

    // 2. Send bulk notifications
    await sendBulkNotifications();

    // 3. List recent notifications
    await listNotifications();

    // 4. Send an OTP via SMS
    await sendOTP();

    // 5. Verify an OTP (replace with real values)
    await verifyOTP("req_placeholder_id", "123456");

    // 6. Upload a file (uncomment and provide a real path)
    // await uploadFile("/path/to/invoice.pdf");

    // 7. List files
    await listFiles();

    // 8. Quick send
    await quickSend();
}

main().catch(console.error);
