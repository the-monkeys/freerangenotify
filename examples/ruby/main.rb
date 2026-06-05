require 'net/http'
require 'uri'
require 'json'

API_KEY = "YOUR_API_KEY"
BASE_URL = "https://freerangenotify.monkeys.support/v1"

def send_notification
  uri = URI.parse("#{BASE_URL}/notifications")
  header = {
    'X-API-Key' => API_KEY,
    'Content-Type' => 'application/json'
  }
  payload = {
    user_id: 'user_123',
    channel: 'email',
    template_id: 'welcome-email',
    data: { name: 'Ruby Developer' }
  }

  http = Net::HTTP.new(uri.host, uri.port)
  http.use_ssl = true
  request = Net::HTTP::Post.new(uri.request_uri, header)
  request.body = payload.to_json

  response = http.request(request)
  puts "Notification sent. Status: #{response.code}"
end

def send_otp
  uri = URI.parse("#{BASE_URL}/otp/send")
  header = { 'X-API-Key' => API_KEY, 'Content-Type' => 'application/json' }
  payload = {
    recipient: 'ruby-user@example.com',
    channel: 'email',
    template_id: 'otp-template'
  }

  http = Net::HTTP.new(uri.host, uri.port)
  http.use_ssl = true
  request = Net::HTTP::Post.new(uri.request_uri, header)
  request.body = payload.to_json

  response = http.request(request)
  puts "OTP sent. Status: #{response.code}"
end

def quick_send
  uri = URI.parse("#{BASE_URL}/quick-send")
  header = { 'X-API-Key' => API_KEY, 'Content-Type' => 'application/json' }
  payload = {
    to: 'ruby-user@example.com',
    msg: 'Hello from Ruby!',
    title: 'Ruby Test'
  }

  http = Net::HTTP.new(uri.host, uri.port)
  http.use_ssl = true
  request = Net::HTTP::Post.new(uri.request_uri, header)
  request.body = payload.to_json

  response = http.request(request)
  puts "Quick Send completed. Status: #{response.code}"
end

puts "FreeRangeNotify Ruby Example"
send_notification
send_otp
quick_send
