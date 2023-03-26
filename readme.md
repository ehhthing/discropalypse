# discropalypse

Scans your Discord data dump for acropalypse affected screenshots at blazing fast speeds!

Uses the WASM module on [https://acropalypse.app/](https://acropalypse.app/) directly.

## Usage

First, you need to request your data dump [using the instructions here](https://support.discord.com/hc/en-us/articles/360004027692-Requesting-a-Copy-of-your-Data).

Run `discropalypse -device {your device} -log {log file} -output {recovered output directory} -package {path to discord data dump .zip file}`

Valid devices are `p3, p3xl, p3a, p3axl, p4, p4xl, p4a, p5, p5a, p6, p6pro, p6a, p7, p7pro` or a custom resolution in form `{width}x{height}`.

```
Usage: discropalypse
  -device string
        either one of [p3, p3xl, p3a, p3axl, p4, p4xl, p4a, p5, p5a, p6, p6pro, p6a, p7, p7pro] or a custom resolution e.g. 1920x1080
  -log string
        location to store download error logs (default "discropalypse.log")
  -output string
        location to store the downloaded and recovered images (default "./download")
  -package string
        path to your discord data dump
  -threads uint
        number of concurrent downloads (default 8)
```