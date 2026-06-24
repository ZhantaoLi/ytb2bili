ytb2bili release package

Contents:
- ytb2bili / ytb2bili.exe
- config.toml
- config.toml.example

Quick start:
1. Edit config.toml only if you need custom paths, proxy, or external APIs.
2. Start ytb2bili.
3. Open http://localhost:8096 in your browser.
4. Complete the first-start wizard to create the first local admin account.

Default behavior:
- Local SQLite database: data/ytb2bili.db
- Local task workspace: ./data
- Automatic upload: disabled
- Account encryption key: auto-generated at data/secrets/account_encryption.key

Optional advanced bootstrap environment variables:
- YTB2BILI_ADMIN_USERNAME
- YTB2BILI_ADMIN_PASSWORD
- YTB2BILI_ADMIN_EMAIL
- YTB2BILI_ACCOUNT_ENCRYPTION_KEY

yt-dlp, ffmpeg, and platform-specific subtitle/upload dependencies still need
to be available on your machine when you use those features.
