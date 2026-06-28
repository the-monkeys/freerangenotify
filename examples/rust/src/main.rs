use reqwest::header::{HeaderMap, HeaderValue, CONTENT_TYPE};
use serde_json::json;
use std::error::Error;

const API_KEY: &str = "YOUR_API_KEY";
const BASE_URL: &str = "https://freerangenotify.monkeys.support/v1";

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    println!("FreeRangeNotify Rust Example");

    // 1. Send a Notification
    send_notification().await?;

    // 2. Send an OTP
    send_otp().await?;

    // 3. Quick Send
    quick_send().await?;

    Ok(())
}

async fn send_notification() -> Result<(), Box<dyn Error>> {
    let url = format!("{}/notifications", BASE_URL);
    let mut headers = HeaderMap::new();
    headers.insert("X-API-Key", HeaderValue::from_str(API_KEY)?);
    headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));

    let client = reqwest::Client::new();
    let payload = json!({
        "user_id": "user_123",
        "channel": "email",
        "template_id": "welcome-email",
        "data": {
            "name": "Rust Developer"
        }
    });

    let res = client.post(url)
        .headers(headers)
        .json(&payload)
        .send()
        .await?;

    println!("Notification sent. Status: {}", res.status());
    Ok(())
}

async fn send_otp() -> Result<(), Box<dyn Error>> {
    let url = format!("{}/otp/send", BASE_URL);
    let mut headers = HeaderMap::new();
    headers.insert("X-API-Key", HeaderValue::from_str(API_KEY)?);

    let client = reqwest::Client::new();
    let payload = json!({
        "recipient": "rust-user@example.com",
        "channel": "email",
        "template_id": "otp-template"
    });

    let res = client.post(url)
        .headers(headers)
        .json(&payload)
        .send()
        .await?;

    println!("OTP sent. Status: {}", res.status());
    Ok(())
}

async fn quick_send() -> Result<(), Box<dyn Error>> {
    let url = format!("{}/quick-send", BASE_URL);
    let mut headers = HeaderMap::new();
    headers.insert("X-API-Key", HeaderValue::from_str(API_KEY)?);

    let client = reqwest::Client::new();
    let payload = json!({
        "to": "rust-user@example.com",
        "msg": "Hello from Rust!",
        "title": "Rust Test"
    });

    let res = client.post(url)
        .headers(headers)
        .json(&payload)
        .send()
        .await?;

    println!("Quick Send completed. Status: {}", res.status());
    Ok(())
}
