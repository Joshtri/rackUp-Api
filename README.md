# RackUp API

Backend untuk [RackUp](../pocket-ball-trackers): akun pengguna, riwayat main, daftar pemain, dan impor backup.

Dibuat dengan Go (`net/http` bawaan), database [Neon](https://neon.com) (Postgres) lewat [pgx](https://github.com/jackc/pgx), dan di-deploy ke [Vercel](https://vercel.com/docs/functions/runtimes/go) (Go runtime, preset server).

> **Kenapa Go, bukan Rust lagi?** Versi Rust (Axum + sqlx) susah di-deploy: Vercel memakai versi `rustc` tertentu yang tidak cocok dengan dependency terbaru. Go runtime Vercel memakai versi Go yang ditulis di `go.mod`, jadi masalah itu hilang. Perilaku API, format data, password hash, token JWT, dan tabel migrasi sama persis dengan versi Rust, jadi database dan akun yang sudah ada tetap jalan.

## Jalankan lokal

Butuh Go 1.26+ dan database Postgres (Neon atau lokal).

```bash
cp .env.example .env                                  # isi DATABASE_URL, DATABASE_URL_UNPOOLED, JWT_SECRET
go run ./cmd/admin migrate                            # buat tabel
go run ./cmd/admin user create joshtri --name "Joshtri"   # buat akun (lihat "Akun")
go run ./cmd/api                                      # http://localhost:8080
go test ./...                                         # unit test (tidak butuh database)
```

## Deploy ke Vercel

1. Buat database di Neon. Ambil 2 connection string: **pooled** (host ada `-pooler`) dan **direct**.
2. Jalankan migrasi dari laptop: `go run ./cmd/admin migrate` (pakai `DATABASE_URL_UNPOOLED`). Migrasi tidak jalan otomatis saat deploy.
3. Import repo ini di Vercel (atau `vercel deploy`). `vercel.json` sudah memilih preset `go`; Vercel membangun `cmd/api/main.go` sebagai server HTTP biasa.
4. Isi Environment Variables di Vercel: `DATABASE_URL` (pooled), `JWT_SECRET`, `CORS_ORIGINS` (domain frontend, contoh `https://rackup.vercel.app`).
   Kalau pakai integrasi Neon di Vercel Marketplace, `DATABASE_URL` dan `DATABASE_URL_UNPOOLED` terisi otomatis.
5. Samakan region function Vercel dengan region Neon (Project → Settings → Functions) supaya query cepat.

Cek: `GET /health` (aplikasi hidup) dan `GET /ready` (database tersambung).

## Endpoint

Semua body dan response JSON. Endpoint bertanda 🔒 butuh header `Authorization: Bearer <token>`.

| Method | Path | Keterangan |
| --- | --- | --- |
| GET | `/health` | Status aplikasi |
| GET | `/ready` | Status koneksi database |
| POST | `/auth/login` | `{ username, password }` → `{ token, expiresAt, user }`. `user.expiresAt` = akhir masa aktif akun |
| GET | `/auth/me` 🔒 | User yang sedang login |
| GET | `/matches` 🔒 | Semua main, tanggal terbaru dulu |
| POST | `/matches` 🔒 | Catat main. `id` dan `createdAt` boleh dikirim (untuk batal hapus) |
| GET | `/matches/{id}` 🔒 | Satu main |
| PUT | `/matches/{id}` 🔒 | Edit `date, playerA, playerB, scoreA, scoreB, note` |
| DELETE | `/matches/{id}` 🔒 | Hapus |
| POST | `/matches/import` 🔒 | `{ matches: [...] }` → `{ added, skipped }`. Digabung per `id`, tidak menimpa |
| GET | `/players` 🔒 | Nama pemain, yang terakhir main paling atas |
| GET | `/users` 🔒 | Akun yang masih aktif: `[{ id, username, displayName }]` (untuk menghubungkan pemain sesi ke akun) |
| GET | `/live-sessions` 🔒 | Sesi shot clock yang sedang jalan (diubah dalam 12 jam terakhir) → `{ serverNow, sessions }` |
| POST | `/live-sessions` 🔒 | `{ id, state }` → buat sesi. `id` dari client; diulang dengan id sama → 200 + data lama |
| GET | `/live-sessions/{id}?since=N` 🔒 | `{ serverNow, session }`. `session` tidak dikirim kalau versinya masih `N` |
| PUT | `/live-sessions/{id}` 🔒 | `{ version, state }`. 409 kalau versi sudah berubah (HP lain duluan) atau sesi sudah berakhir |
| POST | `/live-sessions/{id}/end` 🔒 | `{ match }` simpan hasil ke riwayat (id-nya diganti id sesi), `{ match: null }` buang. Mengakhiri dua kali tidak menyimpan dua kali |

Bentuk data main sama dengan tipe `Match` di frontend (`src/lib/matches.ts`): `date` format `YYYY-MM-DD`, `createdAt` dalam epoch ms, `log` hanya ada untuk main pakai shot clock. Response juga punya `updatedAt`.

Error selalu berbentuk `{ "error": { "code": "...", "message": "..." } }`, contoh kode: `validation_error` (422), `bad_request` (400/413/415/422, body atau URL tidak valid), `unauthorized` (401), `invalid_credentials` (401), `account_expired` (401), `not_found` (404), `conflict` (409).

**Data bersama.** Riwayat, daftar pemain, dan sesi shot clock dipakai bersama oleh semua akun: akun mana pun bisa melihat, mengedit, dan menghapus semuanya. `matches.user_id` hanya mencatat siapa yang menyimpan.

**Sesi bersama (`live_sessions`).** Server menyimpan `state` (objek `Session` dari frontend, `src/lib/session.ts`) apa adanya, dan hanya menjaga urutan perubahan lewat `version`: tiap update harus menyebut versi yang jadi dasarnya, jadi dua HP yang menekan bersamaan tidak saling menimpa (yang kalah dapat 409, ambil data terbaru, lalu terapkan ulang ketukannya). HP mengikuti perubahan dengan polling `GET ?since=` sekitar tiap 1 detik. Semua response sesi berisi `serverNow` (epoch ms) supaya semua HP menghitung shot clock dari jam yang sama.

Aturan yang sama dengan frontend:

- Nama pemain dirapikan (spasi berlebih dibuang). Saat catat main, nama yang sudah ada dipakai ejaannya (`budi` → `Budi`).
- Saat edit, nama yang beda huruf besar/kecil dianggap koreksi dan diganti di semua catatan.
- Kalau posisi pemain A/B ditukar saat edit, `log` shot clock ikut dibalik. `log` sendiri tidak bisa diedit.
- `POST /matches` dengan `id` yang sudah ada mengembalikan data lama (200), jadi batal hapus aman diulang.

Batas: body request maks 4,5 MB (batas Vercel), maks 5.000 main per impor. Backup yang lebih besar dikirim bertahap.

## Akun

Tidak ada endpoint daftar (sign up). Akun dibuat admin lewat CLI, dari laptop yang punya `.env` berisi URL database:

```bash
go run ./cmd/admin user create <username> --name "Nama Tampil" [--days 30]   # password dibuat otomatis, tampil SEKALI
go run ./cmd/admin user list                                                  # status & tanggal kedaluwarsa
go run ./cmd/admin user renew <username> [--days 30]                          # aktif lagi N hari dari sekarang
go run ./cmd/admin user expire <username>                                     # akhiri sekarang
go run ./cmd/admin user reset-password <username>                             # password baru, tampil SEKALI
```

- Username huruf kecil, 3–32 karakter (`a-z 0-9 . _ -`); login tidak peduli huruf besar/kecil.
- Database hanya menyimpan hash Argon2id, bukan password asli.
- Tiap akun punya `expires_at` (default 30 hari sejak dibuat). Login ditolak setelah lewat (`401 account_expired`). Masa berlaku token tidak pernah melewati masa aktif akun. Tiap request ber-🔒 juga membaca `expires_at` dari database, jadi `renew`/`expire` langsung berlaku tanpa menunggu token habis.

Detail alur login di frontend: [`../pocket-ball-trackers/docs/AUTH.md`](../pocket-ball-trackers/docs/AUTH.md).

## Struktur

```
cmd/
  api/main.go            server HTTP (lokal dan Vercel)
  admin/main.go          perintah admin: `migrate`, `user ...`
internal/
  app/                   gabung semua route, CORS, body limit, logging, /health, /ready
  config/                baca env var, setup log
  db/                    pool Postgres (lazy, cocok untuk serverless), jalankan migrasi
  httpx/                 format error JSON, baca/tulis JSON, path param
  validation/            rapikan & cek input
  auth/                  login, me, users, middleware token (Argon2id + JWT), accounts.go = kelola akun
  matches/               CRUD main + impor, tipe log shot clock
  players/               daftar nama pemain
  livesessions/          sesi shot clock bersama antar HP (versi + polling)
migrations/              file SQL, ikut masuk ke binary (embed)
```

Tiap folder fitur berisi `handlers.go` (HTTP) → `service.go` (aturan bisnis) → `repo.go` (query SQL), plus `dto.go` untuk bentuk request/response.

Migrasi dicatat di tabel `_sqlx_migrations` dengan format yang sama seperti sqlx (versi Rust), jadi database lama tidak menjalankan ulang migrasi yang sudah ada. Migrasi baru: tambah file `migrations/<YYYYMMDDHHMMSS>_<nama>.sql`. Jangan ubah file yang sudah dijalankan; checksum-nya dicek.
