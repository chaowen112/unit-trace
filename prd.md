# PRD：UnitTrace MVP

## 1. Product Summary

**UnitTrace** 是一個私人使用的 PropertyGuru 房源追蹤工具。

它由三個部分組成：

```text
Chrome Extension
Backend API
PostgreSQL + local image storage
```

所有安裝 Chrome extension 的人都連到同一個後端，共用同一份房源歷史資料。

核心目標：

> 記住所有看過的房源，追蹤它是否以前看過、是否重新刊登、是否降價、圖片是否重複，以及使用者是否反覆查看。

---

## 2. Product Principle

UnitTrace 不做公開平台，不做多租戶，不做正式帳號系統。

產品原則：

> 一個私人後端，就是共同的房源記憶。

所有資料都是全域共享：

```text
tracked units
listing snapshots
listing visits
price events
images
notes
clients
```

MVP 不需要：

```text
workspace
login
password
permission
public signup
billing
API token
multi-tenant isolation
```

---

## 3. Problem

PropertyGuru 上的 agent 可以重新 post listing，導致買家看不出：

1. 這間房以前是不是看過。
2. 這間房實際上掛多久。
3. Asking price 是否偷偷降過。
4. Listing ID 換掉後，是不是還是同一間。
5. 同一間房是不是被不同 agent 重新包裝。
6. 圖片是不是跟以前的 listing 一樣。
7. 自己和伴侶到底反覆看了哪些房源。

UnitTrace 要解決的核心問題：

> Agent 可以重刊 listing，但 UnitTrace 要認得同一間房。

---

## 4. Target User

MVP 只服務私人使用者，例如：

```text
Charlie
Ellie
```

使用情境：

* 兩個人都安裝 Chrome extension。
* 兩個人都看 PropertyGuru。
* 兩個人的瀏覽紀錄都寫進同一個後端。
* 任一人之後打開同一間房，都可以看到共同歷史。
* Dashboard 可以看到哪些房源被反覆查看、降價、重刊。

---

## 5. MVP Goal

MVP 只做最核心功能：

1. Chrome extension 擷取 PropertyGuru listing。
2. 後端保存房源資料。
3. 每次點開 listing 都新增 visit。
4. 只有房源內容變化時才新增 snapshot。
5. 後端下載並保存 listing 圖片。
6. 判斷 listing 是否以前看過。
7. 判斷 listing 是否疑似重刊。
8. 記錄 price history。
9. 記錄誰看過、看過幾次、最近誰看。
10. 提供 listing overlay。
11. 在 PropertyGuru 搜尋結果頁對已追蹤的 listing 顯示狀態 badge。
12. 提供簡單 dashboard 查看歷史。
13. 支援 notes，並標記是哪個 client 寫的。

---

## 6. Identity Design

MVP 不做 user account。

每個 Chrome extension 安裝實例會自動產生一個 `client_id`：

```text
client_id = crypto.randomUUID()
```

`client_id` 存在：

```text
chrome.storage.local
```

使用者只需要在 extension options page 設定：

```text
Display Name: Charlie
```

或：

```text
Display Name: Ellie
```

每次 extension 上報 snapshot / visit / note 時，都帶上：

```json
{
  "client_id": "generated-uuid",
  "display_name": "Charlie"
}
```

後端用：

```text
client_id     = 穩定識別這個 browser extension instance
display_name  = UI 顯示用名字
```

### 6.1 Identity Rules

* `client_id` 是真正的識別 key。
* `display_name` 可以修改。
* 不需要登入。
* 不需要密碼。
* 不需要 API token。
* 不需要 workspace。
* 同一個人換電腦，會是新的 client。
* 如果改 display name，後端更新 clients table。
* Notes、visits、snapshots 都關聯到 client。

---

## 7. Chrome Extension

### 7.1 Extension Settings

Extension 只需要一個設定：

```text
Display Name
```

Backend URL 寫死在 extension code：

```js
const UNITTRACE_BACKEND_URL = "https://unittrace.example.com";
```

### 7.2 Options Page

Options page 只有一個 input：

```text
Display Name: Charlie
```

儲存到：

```text
chrome.storage.local.display_name
```

第一次安裝時自動建立：

```text
chrome.storage.local.client_id
```

### 7.3 Extension Responsibilities

Extension 負責：

```text
detect PropertyGuru listing page
extract visible listing metadata
extract image URLs
read client_id and display_name
submit page view to backend
render overlay result
detect PropertyGuru search result page
batch-query tracked status for visible listing cards
annotate listing cards with status badges
```

---

## 8. Input Source

### 8.1 PropertyGuru Listing Page

使用者打開 PropertyGuru listing page 時，extension 擷取頁面可見資料。

上報欄位：

```text
client_id
display_name

source
listing_url
listing_id
captured_at

title
asking_price
property_type
project_name
address_text
district
bedrooms
bathrooms
floor_area
floor_level_text
agent_name
agency_name
description_text
image_urls
```

---

### 8.2 PropertyGuru Search Result Page

使用者在搜尋結果頁瀏覽 listing cards 時，extension 批次查詢已追蹤狀態，並在每個 card 上注入 badge。

Extension 從每個 card 擷取：

```text
listing_id（從 card URL 或 data attribute）
```

Badge 類型：

```text
Seen before     → 綠色，顯示 first seen date
Price dropped   → 藍色，顯示降幅
Likely relisted → 橘色
（無）          → 未追蹤，不顯示任何 badge
```

---

## 9. Core Data Concepts

### 9.1 Client

代表一個 Chrome extension 安裝實例。

不是帳號，不是權限系統。

用途：

```text
顯示 first seen by Charlie
顯示 last visited by Ellie
顯示 note author
計算誰看過幾次
```

---

### 9.2 Tracked Unit

**Tracked Unit** 代表 UnitTrace 認定的「同一間房」。

PropertyGuru listing 可能被重新 post，listing ID 可能改，標題可能改，價格可能改，但 UnitTrace 要盡量判斷它是不是同一間實體房源。

Example：

```text
Tracked Unit: Example Condo, 2BR, 732 sqft, high floor

Snapshots:
- Listing ID 12345, seen on 2026-04-10, S$1,250,000
- Listing ID 12345, seen on 2026-05-01, S$1,250,000
- Listing ID 67890, seen on 2026-06-17, S$1,180,000
```

Tracked Unit 儲存長期狀態：

```text
first_seen_at
last_seen_at
last_visited_at
first_seen_by_client_id
last_seen_by_client_id
last_visited_by_client_id
first_seen_price
current_price
lowest_seen_price
highest_seen_price
possible_relist_count
snapshot_count
visit_count
```

使用者真正關心的是 Tracked Unit，不是單次 snapshot。

---

### 9.3 Listing Snapshot

**Listing Snapshot** 代表某個時間點房源內容的版本。

Snapshot 不是每次點開都新增。

只有內容變化時才新增，例如：

```text
price changed
title changed
description changed
agent changed
image set changed
listing_id changed
floor_area changed
property fields changed
```

如果使用者今天點開同一個 listing 10 次，但內容完全沒變，UnitTrace 不新增 10 筆 snapshots。

只新增 visits，並更新：

```text
last_visited_at
last_visited_by_client_id
visit_count
```

---

### 9.4 Listing Visit

**Listing Visit** 代表某個 client 在某個時間點打開過某個 listing。

每次打開 listing 都新增一筆 visit。

Visit 用來回答：

```text
誰看過這間？
誰最近看？
Charlie 看過幾次？
Ellie 看過幾次？
這間是不是被反覆打開？
```

這可以變成 interest signal。

Example：

```text
Charlie viewed 8 times
Ellie viewed 4 times
Last viewed today by Charlie
```

---

### 9.5 Price Event

價格變化時產生 event。

Event types：

```text
first_seen
same_price_seen_again
price_decreased
price_increased
possible_relist
```

---

### 9.6 Image

後端下載圖片並保存。

同一張圖片只存一次，用 `sha256_hash` 去重。

圖片用途：

1. 人工回看舊 listing。
2. 判斷新舊 listing 是否同一間。
3. 找出 listing ID 換掉但圖片相同的重刊。
4. 顯示同一間房過去出現過哪些照片。

---

### 9.7 Note

使用者可以在 Tracked Unit 上寫 note。

Note 需要記錄：

```text
tracked_unit_id
author_client_id
note
created_at
```

UI 顯示：

```text
Charlie: Agent said seller seems flexible.
Ellie: Layout looks bad, kitchen too small.
```

---

## 10. Main Flow

### 10.1 Listing Page Flow

```text
User opens PropertyGuru listing
↓
Chrome extension extracts metadata and image URLs
↓
Extension sends page view to backend
↓
Backend upserts client
↓
Backend finds exact match by listing_id / canonical_url
↓
If no exact match, backend runs fuzzy match
↓
Backend creates or updates Tracked Unit
↓
Backend creates Listing Visit
↓
Backend compares current content with latest snapshot
↓
If content changed, backend creates new Listing Snapshot
↓
Backend downloads new images
↓
Backend calculates image hashes
↓
Backend updates price history
↓
Backend returns result
↓
Extension shows overlay
```

---

## 11. Snapshot vs Visit Rules

### 11.1 Always Create Visit

Every page view creates one visit record.

```text
User opens listing
↓
create listing_visit
```

### 11.2 Only Create Snapshot If Content Changed

A new snapshot is created only if current captured content differs from latest snapshot for the same Tracked Unit.

Compare fields：

```text
listing_id
canonical_url
title
asking_price
property_type
project_name
address_text
district
bedrooms
bathrooms
floor_area
floor_level_text
agent_name
agency_name
description_hash
image_set_hash
```

If none changed：

```text
do not create new snapshot
update tracked_unit.last_visited_at
update tracked_unit.visit_count
```

If changed：

```text
create new snapshot
create relevant price event if price changed
update tracked_unit current state
```

---

## 12. Overlay UI

### 12.1 New Listing

```text
UnitTrace

New to database
Current price: S$1,180,000 (S$1,612/sqft)
Captured by: Charlie
```

---

### 12.2 Seen Before

```text
UnitTrace

Seen before
First seen: 2026-04-10 by Ellie
Last visited: today by Charlie

Visits:
Charlie: 8
Ellie: 4

First price: S$1,250,000 (S$1,708/sqft)
Current price: S$1,180,000 (S$1,612/sqft)
Change: -S$70,000 / -5.6%

Content versions: 3
```

---

### 12.3 Likely Relisted

```text
UnitTrace

Likely relisted
Confidence: 88%

First seen: 68 days ago
Old listing ID: 12345
Current listing ID: 67890

Price drop: -S$70,000 / -5.6%
Current psf: S$1,612/sqft
Matched images: 12

Visits:
Charlie: 8
Ellie: 4
```

---

### 12.4 Possible Duplicate

```text
UnitTrace

Possible duplicate
Confidence: 66%

This listing looks similar to a previously seen unit.
Not auto-merged.

Reason:
Same project
Similar floor area
Same bedroom count
```

---

## 13. Match Logic

### 13.1 Exact Match

如果以下任一條件成立，視為同一個 listing：

```text
same source + same listing_id
same source + same canonical_url
```

Result：

```text
status = seen_before
confidence = 100
```

---

### 13.2 Fuzzy Relist Match

當 listing ID 或 URL 不同時，用 scoring 判斷是否同一間。

Match score：

```text
same project / block: +30
same district: +5
same property type: +10
same bedrooms: +10
same bathrooms: +5
floor area within 3%: +20
same floor level text: +10
same agent: +5
similar title: +5
similar description: +15
same image sha256: +40
similar image phash: +25
price within 15%: +5
psf within 10%: +10
```

`similar description` 使用 PostgreSQL `pg_trgm` trigram similarity。threshold 為 0.4（40% trigram 重疊視為相似）。description 為空時，此項得 0 分。

`similar title` 同樣使用 trigram，threshold 0.5。

判斷：

```text
score >= 90:
  almost_certain_same_unit

score 75-89:
  likely_same_unit

score 60-74:
  possible_duplicate

score < 60:
  new_listing
```

MVP 自動合併：

```text
score >= 75
```

MVP 不自動合併：

```text
score 60-74
```

`60-74` 只顯示 possible duplicate，之後可以人工 merge。

---

## 14. Image Handling

### 14.1 Extension Extracts Image URLs

Extension 從 listing page 擷取：

```text
image_urls
image_order
```

---

### 14.2 Backend Downloads Images

後端收到 image URLs 後，**非同步**下載圖片。

API response 在 visit / snapshot / match 完成後立即回傳，不等待圖片下載。圖片下載在背景處理。

這代表新 listing 的 `matched_image_count` 在 response 裡可能為 0，等背景下載完成後才會更新。

下載流程：

```text
download image
↓
calculate sha256
↓
calculate phash
↓
check duplicate by sha256
↓
store file on disk if new
↓
create snapshot-image relation
↓
recalculate image_set_hash for snapshot
```

---

### 14.3 Image Storage

圖片不要存 PostgreSQL binary。

使用 local disk：

```text
/data/unittrace/images/
  ab/
    abcd1234efgh.jpg
  cd/
    cdef5678ijkl.jpg
```

PostgreSQL 只存 image metadata。

---

### 14.4 Image Set Hash

每個 snapshot 產生一個 `image_set_hash`：

```text
sorted(image_sha256_hashes) → hash
```

用途：

```text
判斷圖片集合是否變化
避免同內容重複產生 snapshot
判斷 relist 是否使用同一組圖片
```

---

## 15. Interest Signal

UnitTrace 不做正式推薦模型，但可以用簡單規則顯示 interest signal。

### 15.1 Metrics

```text
visit_count
unique_client_count
recent_visit_count_7d
note_count
both_clients_viewed
```

### 15.2 Basic Labels

```text
High interest:
  visit_count >= 8
  OR both clients viewed and visit_count >= 5
  OR note_count >= 2

Medium interest:
  visit_count >= 3

Low interest:
  visit_count < 3
```

### 15.3 Dashboard Display

```text
Interest: High
Visits: 12
Viewed by: Charlie, Ellie
Last viewed: today by Charlie
```

---

## 16. Backend API

### 16.1 Submit Listing View

```http
POST /api/v1/listing-views
```

Request：

```json
{
  "client_id": "550e8400-e29b-41d4-a716-446655440000",
  "display_name": "Charlie",

  "source": "propertyguru",
  "listing_url": "https://www.propertyguru.com.sg/...",
  "listing_id": "12345678",
  "captured_at": "2026-06-17T10:30:00+08:00",

  "title": "High Floor 2 Bedroom Near MRT",
  "asking_price": 1180000,
  "property_type": "condo",
  "project_name": "Example Residences",
  "address_text": "Example Road",
  "district": "D09",
  "bedrooms": 2,
  "bathrooms": 2,
  "floor_area": 732,
  "floor_level_text": "High Floor",
  "agent_name": "John Tan",
  "agency_name": "ABC Realty",
  "description_text": "Rare high floor unit...",
  "image_urls": [
    "https://..."
  ]
}
```

Response：

```json
{
  "status": "likely_relisted",
  "tracked_unit_id": 123,
  "match_confidence": 88,

  "visit_created": true,
  "snapshot_created": true,

  "first_seen_at": "2026-04-10T09:20:00+08:00",
  "last_seen_at": "2026-06-17T10:30:00+08:00",
  "last_visited_at": "2026-06-17T10:30:00+08:00",

  "first_seen_by": "Ellie",
  "last_seen_by": "Charlie",
  "last_visited_by": "Charlie",

  "first_seen_price": 1250000,
  "current_price": 1180000,
  "price_change": -70000,
  "price_change_pct": -5.6,

  "possible_relist_count": 2,
  "snapshot_count": 3,
  "visit_count": 12,

  "client_visit_counts": [
    {
      "display_name": "Charlie",
      "visit_count": 8
    },
    {
      "display_name": "Ellie",
      "visit_count": 4
    }
  ],

  "matched_image_count": 12,

  "match_reasons": [
    "same project",
    "same bedroom count",
    "floor area within 3%",
    "similar description",
    "matched images"
  ]
}
```

---

### 16.2 Get Tracked Unit

```http
GET /api/v1/tracked-units/{tracked_unit_id}
```

Response includes：

```text
tracked unit summary
all snapshots
all visits
price timeline
all listing URLs
all agents
all images
notes
match reasons
client visit counts
```

---

### 16.3 List Tracked Units

```http
GET /api/v1/tracked-units
```

Query params：

```text
price_dropped=true
likely_relisted=true
min_seen_days=30
min_visit_count=5
captured_by_client_id=1
visited_by_client_id=1
interest=high
```

---

### 16.4 Add Note

```http
POST /api/v1/tracked-units/{tracked_unit_id}/notes
```

Request：

```json
{
  "client_id": "550e8400-e29b-41d4-a716-446655440000",
  "display_name": "Charlie",
  "note": "Agent said seller is flexible. Can try 1.12M."
}
```

---

### 16.5 Batch Listing Status

供 search result page badge 使用。

```http
POST /api/v1/listing-status-batch
```

Request：

```json
{
  "source": "propertyguru",
  "listing_ids": ["12345", "67890", "11111"]
}
```

Response：

```json
{
  "results": {
    "12345": {
      "status": "seen_before",
      "first_seen_at": "2026-04-10T09:20:00+08:00",
      "current_price": 1180000,
      "price_change_pct": -5.6
    },
    "67890": {
      "status": "likely_relisted"
    },
    "11111": {
      "status": "not_tracked"
    }
  }
}
```

Status 值：

```text
not_tracked
seen_before
likely_relisted
```

---

## 17. Database Schema

### 17.1 clients

```sql
CREATE TABLE clients (
  id BIGSERIAL PRIMARY KEY,

  client_id UUID NOT NULL UNIQUE,
  display_name TEXT NOT NULL DEFAULT 'Unknown',

  first_seen_at TIMESTAMPTZ DEFAULT now(),
  last_seen_at TIMESTAMPTZ DEFAULT now(),

  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);
```

---

### 17.2 tracked_units

```sql
CREATE TABLE tracked_units (
  id BIGSERIAL PRIMARY KEY,

  canonical_name TEXT,
  source TEXT,
  property_type TEXT,
  project_name TEXT,
  address_text TEXT,
  district TEXT,

  first_seen_at TIMESTAMPTZ,
  last_seen_at TIMESTAMPTZ,
  last_visited_at TIMESTAMPTZ,

  first_seen_by_client_id BIGINT REFERENCES clients(id),
  last_seen_by_client_id BIGINT REFERENCES clients(id),
  last_visited_by_client_id BIGINT REFERENCES clients(id),

  first_seen_price BIGINT,
  current_price BIGINT,
  lowest_seen_price BIGINT,
  highest_seen_price BIGINT,

  possible_relist_count INT DEFAULT 0,
  snapshot_count INT DEFAULT 0,
  visit_count INT DEFAULT 0,

  interest_label TEXT,

  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);
```

---

### 17.3 listing_snapshots

```sql
CREATE TABLE listing_snapshots (
  id BIGSERIAL PRIMARY KEY,

  -- tracked_unit 的關聯透過 tracked_unit_snapshots junction table，不在此放 FK，
  -- 以便支援日後 merge / unmerge 操作。

  source TEXT NOT NULL,
  listing_url TEXT NOT NULL,
  canonical_url TEXT,
  listing_id TEXT,

  captured_at TIMESTAMPTZ NOT NULL,
  captured_by_client_id BIGINT REFERENCES clients(id),

  title TEXT,
  asking_price BIGINT,  -- 單位：SGD 全額（例如 1180000 = S$1,180,000）
  property_type TEXT,
  project_name TEXT,
  address_text TEXT,
  district TEXT,
  bedrooms INT,
  bathrooms INT,
  floor_area NUMERIC(8,2),  -- sqft，保留小數位
  floor_level_text TEXT,
  agent_name TEXT,
  agency_name TEXT,

  description_text TEXT,
  description_hash TEXT,

  image_set_hash TEXT,
  content_hash TEXT,
  normalized_fingerprint TEXT,

  raw_payload JSONB,

  created_at TIMESTAMPTZ DEFAULT now()
);
```

---

### 17.4 listing_visits

```sql
CREATE TABLE listing_visits (
  id BIGSERIAL PRIMARY KEY,

  tracked_unit_id BIGINT NOT NULL REFERENCES tracked_units(id),
  snapshot_id BIGINT REFERENCES listing_snapshots(id),
  client_id BIGINT REFERENCES clients(id),

  source TEXT NOT NULL,
  listing_url TEXT NOT NULL,
  listing_id TEXT,

  visited_at TIMESTAMPTZ NOT NULL DEFAULT now(),

  created_at TIMESTAMPTZ DEFAULT now()
);
```

---

### 17.5 tracked_unit_snapshots

這是 snapshot 與 tracked unit 之間的**唯一**關聯表。

`listing_snapshots` 不直接持有 `tracked_unit_id`，所有關聯都透過這張表。這讓日後 merge / unmerge 只需要操作 junction table，不需要修改 snapshot 本身。

同時記錄這個 snapshot 是如何被歸到這個 tracked unit 的（match 原因與分數）。

```sql
CREATE TABLE tracked_unit_snapshots (
  id BIGSERIAL PRIMARY KEY,

  tracked_unit_id BIGINT NOT NULL REFERENCES tracked_units(id),
  snapshot_id BIGINT NOT NULL REFERENCES listing_snapshots(id),

  match_type TEXT NOT NULL,
  match_score INT,
  match_reasons JSONB,

  created_at TIMESTAMPTZ DEFAULT now(),

  UNIQUE (tracked_unit_id, snapshot_id)
);
```

---

### 17.6 price_events

```sql
CREATE TABLE price_events (
  id BIGSERIAL PRIMARY KEY,

  tracked_unit_id BIGINT NOT NULL REFERENCES tracked_units(id),
  snapshot_id BIGINT NOT NULL REFERENCES listing_snapshots(id),

  event_type TEXT NOT NULL,
  old_price BIGINT,
  new_price BIGINT,
  price_delta BIGINT,
  price_delta_pct NUMERIC,

  detected_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now()
);
```

---

### 17.7 images

```sql
CREATE TABLE images (
  id BIGSERIAL PRIMARY KEY,

  original_url TEXT,
  storage_path TEXT NOT NULL,

  sha256_hash TEXT NOT NULL UNIQUE,
  phash TEXT,

  width INT,
  height INT,
  content_type TEXT,
  file_size BIGINT,

  first_seen_at TIMESTAMPTZ DEFAULT now(),
  created_at TIMESTAMPTZ DEFAULT now()
);
```

---

### 17.8 listing_snapshot_images

```sql
CREATE TABLE listing_snapshot_images (
  id BIGSERIAL PRIMARY KEY,

  snapshot_id BIGINT NOT NULL REFERENCES listing_snapshots(id),
  image_id BIGINT NOT NULL REFERENCES images(id),

  sort_order INT,
  original_url TEXT,

  created_at TIMESTAMPTZ DEFAULT now(),

  UNIQUE (snapshot_id, image_id)
);
```

---

### 17.9 notes

```sql
CREATE TABLE notes (
  id BIGSERIAL PRIMARY KEY,

  tracked_unit_id BIGINT NOT NULL REFERENCES tracked_units(id),
  author_client_id BIGINT REFERENCES clients(id),

  note TEXT NOT NULL,

  created_at TIMESTAMPTZ DEFAULT now()
);
```

---

## 18. Content Hash Rules

To determine whether a new snapshot is needed, backend calculates `content_hash`.

Fields included：

```text
listing_id
canonical_url
title
asking_price
property_type
project_name
address_text
district
bedrooms
bathrooms
floor_area
floor_level_text
agent_name
agency_name
description_hash
image_set_hash
```

Pseudo logic：

```text
latest_snapshot = most recent snapshot linked to tracked unit
                  (via tracked_unit_snapshots, ordered by captured_at DESC)

if latest_snapshot.content_hash == current_content_hash:
  create visit only
  do not create snapshot
else:
  create new snapshot
  link snapshot to tracked unit via tracked_unit_snapshots
  create price event if price changed
  update tracked unit
```

注意：`content_hash` 包含 `listing_id`，因此 relist（listing_id 換掉）一定產生新 snapshot，這是正確行為——新的 listing_id 本身就代表內容變化。Snapshot 去重（避免同一個 listing_id 重複點開產生多筆）與 relist 判斷是兩個獨立的邏輯，不互相干擾。

---

## 19. Backend Components

### 19.1 API Server

Responsibilities：

```text
receive listing views
upsert clients
normalize fields
find or create tracked unit
create visits
create snapshots only when content changes
download images
calculate image hashes
match listings
update price events
serve dashboard data
handle notes
```

Recommended stack：

```text
Go + PostgreSQL + local disk image storage
```

---

### 19.2 Image Worker

MVP 直接內建在 API server，以 goroutine 非同步執行，不阻擋 API response。

Responsibilities：

```text
download image URL
calculate sha256
calculate phash
deduplicate image
save to disk
create DB relation
recalculate image_set_hash for snapshot
```

API response 不等待圖片下載完成。圖片下載在 response 回傳後於背景執行。

---

### 19.3 Matching Engine

Responsibilities：

```text
exact match by listing ID / URL
fuzzy match by property fields
image match by sha256
image similarity by phash
score calculation
tracked unit creation
tracked unit update
match reason generation
```

---

### 19.4 Price Event Engine

Responsibilities：

```text
compare new snapshot price with tracked unit current price
create price_decreased / price_increased event
update first/current/lowest/highest price
update last_seen_at
```

---

### 19.5 Visit Engine

Responsibilities：

```text
create listing visit for every page view
update tracked_unit.visit_count
update tracked_unit.last_visited_at
update tracked_unit.last_visited_by_client_id
calculate per-client visit count
update interest label
```

---

## 20. Dashboard

MVP dashboard 可以很陽春，但要能查資料。

### 20.1 Tracked Unit List Page

Columns：

```text
property
current_price
first_seen_at
seen_days
first_seen_by
last_seen_by
last_visited_by
visit_count
interest_label
price_change
price_change_pct
possible_relist_count
snapshot_count
last_seen_at
last_visited_at
```

Filters：

```text
price dropped
likely relisted
seen > 30 days
seen > 60 days
visited > 5 times
interest = high
visited by Charlie
visited by Ellie
```

---

### 20.2 Tracked Unit Detail Page

Shows：

```text
summary
price timeline
visit timeline
client visit counts
all snapshots
all listing URLs
all agents
all images
matched image count
description changes
notes
```

---

### 20.3 Image Gallery

For each tracked unit：

```text
show all unique images
show which snapshots used each image
show image first seen date
```

---

### 20.4 Notes

Tracked unit detail page 支援新增 notes。

每個 note 顯示：

```text
author display name
created_at
note content
```

---

## 21. MVP Cutline

### 21.1 Must Have

```text
Chrome extension
display name options page
auto-generated client_id
fixed backend URL
listing view submission API
PostgreSQL schema
clients table
tracked units
listing snapshots
listing visits
price events
image download
image storage on disk
image dedupe by sha256
basic phash
content_hash
image_set_hash
exact match
fuzzy match
overlay result
search result page badge
batch listing status API
basic dashboard
notes
```

---

### 21.2 Nice to Have

```text
manual merge / unmerge
scheduled recrawl
Telegram notification
CSV export
MinIO
better dashboard UI
visit heatmap
interest score tuning
```

---

### 21.3 Not in MVP

```text
workspace
login
password
API token
permissions
billing
public signup
mobile app
valuation model
transaction comps
offer recommendation
full crawler
```

---

## 22. Implementation Order

### Phase 1：Backend Core

```text
PostgreSQL schema
clients upsert
submit listing view API
exact match
tracked unit creation
visit creation
snapshot creation only when content changes
price event creation
basic JSON response
```

---

### Phase 2：Chrome Extension

```text
fixed backend URL
auto client_id generation
display name options page
PropertyGuru listing parser
submit listing view
show overlay (with psf display)
PropertyGuru search result parser
batch-query listing status
show status badges on listing cards
```

---

### Phase 3：Images

```text
extract image URLs
backend image download
sha256 dedupe
phash calculation
store files on disk
image relation table
image_set_hash
```

---

### Phase 4：Relist Detection

```text
field-based fuzzy matching
description similarity
image hash matching
match score
likely relisted status
possible duplicate status
match reasons
```

---

### Phase 5：Dashboard

```text
tracked unit list
tracked unit detail
price timeline
visit timeline
client visit counts
image gallery
notes
filters
```

---

## 23. Success Criteria

MVP 成功標準：

1. Charlie 看過的 listing，Ellie 打開時能看到 shared history。
2. Same listing ID 再次出現時，能顯示 seen before。
3. Listing ID 換掉但圖片相同時，能顯示 likely relisted。
4. 每次打開 listing 都會新增 visit。
5. Listing 內容沒變時，不會新增多餘 snapshot。
6. Listing 價格變化時，會新增 snapshot 和 price event。
7. 圖片可以回看，且相同圖片不重複存檔。
8. Dashboard 可以看出：

   * first seen date
   * last seen date
   * last visited date
   * first seen by
   * last seen by
   * last visited by
   * visit count
   * per-client visit count
   * price drop
   * relist count
   * image history
   * notes
9. 使用者可以在 3 秒內知道：

   * 這間以前有沒有看過
   * 第一次何時看到
   * 誰先看到
   * 最近誰看
   * 看過幾次
   * 價格有沒有變
   * 是否疑似重刊

---

## 24. Final Definition

UnitTrace MVP 是：

> 一個 single-backend、shared-history、client-identified、image-aware、visit-aware 的私人房源追蹤系統。

它不需要帳號。
它不需要 workspace。
它不需要 token。
它只需要一個後端、一個資料庫、一個圖片目錄，以及幾個 Chrome extension client。

核心資料語意：

```text
Tracked Unit
= UnitTrace 認定的同一間房

Listing Snapshot
= 某個時間點，這間房源內容的版本

Listing Visit
= 某個人某個時間點打開看過
```

最重要的產品價值：

> 不只知道房源有沒有重刊，也知道你們其實一直在看哪幾間。
