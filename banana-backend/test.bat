#!/bin/bash
set -e

BASE="http://localhost:8080"

echo "=== 1. Healthcheck ==="
curl -s "$BASE/healthz"
echo

echo
echo "=== 2. Register ==="
curl -s -X POST "$BASE/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"secret123"}'
echo

echo
echo "=== 3. Login (save token) ==="
TOKEN=$(curl -s -X POST "$BASE/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"secret123"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

echo "TOKEN=$TOKEN"

echo
echo "=== 4. Создать тестовые файлы на диске ==="
echo "Hello from Banana!" > test1.txt
dd if=/dev/urandom of=test2.bin bs=1024 count=50 2>/dev/null
echo '{"name":"banana","version":1}' > test3.json

echo
echo "=== 5. Upload file 1 (txt) ==="
curl -s -X POST "$BASE/api/v1/files" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@test1.txt"
echo

echo
echo "=== 6. Upload file 2 (bin) ==="
curl -s -X POST "$BASE/api/v1/files" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@test2.bin"
echo

echo
echo "=== 7. Upload file 3 (json) ==="
UPLOAD3=$(curl -s -X POST "$BASE/api/v1/files" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@test3.json")
echo "$UPLOAD3"
FILE_ID=$(echo "$UPLOAD3" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
echo
echo "Last uploaded file id: $FILE_ID"

echo
echo "=== 8. List files ==="
curl -s "$BASE/api/v1/files" -H "Authorization: Bearer $TOKEN"
echo

echo
echo "=== 9. Get file info ==="
curl -s "$BASE/api/v1/files/$FILE_ID" -H "Authorization: Bearer $TOKEN"
echo

echo
echo "=== 10. Download file ==="
curl -s -o downloaded.json \
  "$BASE/api/v1/files/$FILE_ID/download" \
  -H "Authorization: Bearer $TOKEN"
echo "Saved to downloaded.json:"
cat downloaded.json
echo

echo
echo "=== 11. Delete file ==="
curl -s -X DELETE "$BASE/api/v1/files/$FILE_ID" \
  -H "Authorization: Bearer $TOKEN"
echo

echo
echo "=== 12. List files after delete ==="
curl -s "$BASE/api/v1/files" -H "Authorization: Bearer $TOKEN"
echo

echo
echo "=== 13. Logout ==="
curl -s -X POST "$BASE/api/v1/auth/logout" \
  -H "Authorization: Bearer $TOKEN"
echo

echo
echo "=== 14. Unauthorized access (should fail 401) ==="
curl -s -w "\nHTTP code: %{http_code}\n" "$BASE/api/v1/files" \
  -H "Authorization: Bearer invalid-token"

echo
echo "=== Done ==="

::для теста
::chmod +x test.sh
::./test.sh