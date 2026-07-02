resource "matomo_site" "main" {
  name     = "My Website"
  urls     = ["https://www.example.com"]
  timezone = "America/New_York"
  currency = "USD"
}
