package benchmark

import (
	"fmt"
	"strings"
)

// User represents a user record for benchmark data
type User struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Age      int      `json:"age"`
	Active   bool     `json:"active"`
	Score    float64  `json:"score"`
	Profile  Profile  `json:"profile"`
	Settings Settings `json:"settings"`
}

// Profile represents user profile information
type Profile struct {
	Bio     string  `json:"bio"`
	Avatar  string  `json:"avatar"`
	Address Address `json:"address"`
}

// Address represents a physical address
type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	Country string `json:"country"`
	Zip     string `json:"zip"`
}

// Settings represents user settings
type Settings struct {
	Notifications bool        `json:"notifications"`
	Theme         string      `json:"theme"`
	Language      string      `json:"language"`
	Preferences   Preferences `json:"preferences"`
}

// Preferences represents user preferences
type Preferences struct {
	DarkMode    bool   `json:"darkMode"`
	FontSize    int    `json:"fontSize"`
	ColorScheme string `json:"colorScheme"`
}

// Deterministic string generators using simple hash functions
func generateName(i int) string {
	firstNames := []string{"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank", "Grace", "Henry", "Ivy", "Jack"}
	lastNames := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez"}
	return firstNames[i%len(firstNames)] + " " + lastNames[(i*7)%len(lastNames)]
}

func generateCity(i int) string {
	cities := []string{"New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Philadelphia", "San Antonio", "San Diego", "Dallas", "Austin"}
	return cities[i%len(cities)]
}

func generateCountry(i int) string {
	countries := []string{"USA", "Canada", "UK", "Germany", "France", "Australia", "Japan", "Brazil", "India", "Mexico"}
	return countries[i%len(countries)]
}

func generateTheme(i int) string {
	themes := []string{"light", "dark", "system", "custom"}
	return themes[i%len(themes)]
}

func generateColorScheme(i int) string {
	schemes := []string{"default", "ocean", "forest", "sunset", "midnight"}
	return schemes[i%len(schemes)]
}

// GenerateLargeJSON creates a large JSON document with the specified number of users.
// For count=50000, this generates approximately 16MB of JSON data.
func GenerateLargeJSON(count int) []byte {
	var sb strings.Builder
	sb.Grow(count * 350) // Approximate size per user

	sb.WriteString(`{"users":[`)

	for i := 0; i < count; i++ {
		if i > 0 {
			sb.WriteString(",")
		}

		user := fmt.Sprintf(`{"id":%d,"name":"%s","email":"user%d@example.com","age":%d,"active":%t,"score":%.2f,"profile":{"bio":"User %d biography with some longer text to increase size","avatar":"https://avatars.example.com/user%d.png","address":{"street":"%d Main Street","city":"%s","country":"%s","zip":"%05d"}},"settings":{"notifications":%t,"theme":"%s","language":"en","preferences":{"darkMode":%t,"fontSize":%d,"colorScheme":"%s"}}}`,
			i,
			generateName(i),
			i,
			18+(i%62), // Ages 18-79
			i%3 != 0,  // 2/3 active
			float64(50+(i%50))+float64(i%100)/100.0,
			i,
			i,
			100+(i%900),
			generateCity(i),
			generateCountry(i),
			10000+(i%90000),
			i%2 == 0,
			generateTheme(i),
			i%4 == 0,
			12+(i%8),
			generateColorScheme(i),
		)
		sb.WriteString(user)
	}

	sb.WriteString(`]}`)
	return []byte(sb.String())
}

// GenerateHugeJSON creates a 100MB+ JSON document with complex nested data.
// For count=300000, this generates approximately 130MB of JSON data.
func GenerateHugeJSON(count int) []byte {
	var sb strings.Builder
	sb.Grow(count * 450) // Larger estimate per record

	sb.WriteString(`{"dataset":{"version":"2.0","generated":"2026-01-10","source":"benchmark"},"records":[`)

	tags := []string{"premium", "verified", "active", "new", "featured", "trending", "popular", "recommended"}
	categories := []string{"tech", "finance", "health", "education", "entertainment", "sports", "travel", "food"}

	for i := 0; i < count; i++ {
		if i > 0 {
			sb.WriteString(",")
		}

		// More complex record with arrays and nested objects
		record := fmt.Sprintf(`{"id":%d,"uuid":"uuid-%d-%d-%d","name":"%s","email":"user%d@company.com","age":%d,"salary":%.2f,"active":%t,"tags":["%s","%s","%s"],"category":"%s","metadata":{"created":"2026-01-0%d","updated":"2026-01-10","views":%d,"likes":%d,"shares":%d},"profile":{"bio":"User %d professional biography with extensive description for benchmark testing purposes","avatar":"https://cdn.example.com/avatars/%d.jpg","location":{"city":"%s","country":"%s","lat":%.4f,"lng":%.4f},"social":{"twitter":"@user%d","linkedin":"user%d","github":"user%d"}},"preferences":{"theme":"%s","language":"en","notifications":%t,"marketing":%t}}`,
			i,
			i, i*7, i*13,
			generateName(i),
			i,
			18+(i%62),
			float64(30000+(i%70000))+float64(i%100)/100.0,
			i%3 != 0,
			tags[i%len(tags)], tags[(i+3)%len(tags)], tags[(i+5)%len(tags)],
			categories[i%len(categories)],
			1+(i%9),
			100+(i%10000),
			i%500,
			i%100,
			i,
			i,
			generateCity(i),
			generateCountry(i),
			float64(25+(i%40))+float64(i%100)/100.0,
			float64(-120+(i%240))+float64(i%100)/100.0,
			i, i, i,
			generateTheme(i),
			i%2 == 0,
			i%5 == 0,
		)
		sb.WriteString(record)
	}

	sb.WriteString(`]}`)
	return []byte(sb.String())
}

// GenerateArrayTestData creates data optimized for testing array modifiers
func GenerateArrayTestData(arraySize int) []byte {
	var sb strings.Builder
	sb.Grow(arraySize * 100)

	sb.WriteString(`{"numbers":[`)
	for i := 0; i < arraySize; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("%d", i))
	}
	sb.WriteString(`],"booleans":[`)
	for i := 0; i < arraySize; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		if i%3 == 0 {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	}
	sb.WriteString(`],"strings":[`)
	for i := 0; i < arraySize; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`"item%d"`, i))
	}
	sb.WriteString(`],"objects":[`)
	for i := 0; i < arraySize; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`{"id":%d,"value":"v%d","active":%t}`, i, i, i%2 == 0))
	}
	sb.WriteString(`]}`)
	return []byte(sb.String())
}

// GenerateLargeJSONWithMetadata creates JSON with additional metadata fields
func GenerateLargeJSONWithMetadata(count int) []byte {
	var sb strings.Builder
	sb.Grow(count*350 + 200)

	sb.WriteString(`{"metadata":{"generated":"2026-01-09","version":"1.0","count":`)
	sb.WriteString(fmt.Sprintf("%d", count))
	sb.WriteString(`},"users":[`)

	for i := 0; i < count; i++ {
		if i > 0 {
			sb.WriteString(",")
		}

		user := fmt.Sprintf(`{"id":%d,"name":"%s","email":"user%d@example.com","age":%d,"active":%t,"score":%.2f,"profile":{"bio":"User %d biography with some longer text to increase size","avatar":"https://avatars.example.com/user%d.png","address":{"street":"%d Main Street","city":"%s","country":"%s","zip":"%05d"}},"settings":{"notifications":%t,"theme":"%s","language":"en","preferences":{"darkMode":%t,"fontSize":%d,"colorScheme":"%s"}}}`,
			i,
			generateName(i),
			i,
			18+(i%62),
			i%3 != 0,
			float64(50+(i%50))+float64(i%100)/100.0,
			i,
			i,
			100+(i%900),
			generateCity(i),
			generateCountry(i),
			10000+(i%90000),
			i%2 == 0,
			generateTheme(i),
			i%4 == 0,
			12+(i%8),
			generateColorScheme(i),
		)
		sb.WriteString(user)
	}

	sb.WriteString(`]}`)
	return []byte(sb.String())
}

// DataSizeInfo holds information about generated data sizes
type DataSizeInfo struct {
	Bytes       int
	KB          float64
	MB          float64
	ExceedsL1   bool // L1 cache typically 32KB
	ExceedsL2   bool // L2 cache typically 256KB
	ExceedsL3   bool // L3 cache typically 8-16MB
	Description string
}

// GetDataSizeInfo returns size information for the given data
func GetDataSizeInfo(data []byte) DataSizeInfo {
	bytes := len(data)
	kb := float64(bytes) / 1024
	mb := kb / 1024

	info := DataSizeInfo{
		Bytes:     bytes,
		KB:        kb,
		MB:        mb,
		ExceedsL1: bytes > 32*1024,
		ExceedsL2: bytes > 256*1024,
		ExceedsL3: bytes > 8*1024*1024,
	}

	if info.ExceedsL3 {
		info.Description = fmt.Sprintf("%.2f MB (exceeds L3 cache)", mb)
	} else if info.ExceedsL2 {
		info.Description = fmt.Sprintf("%.2f KB (exceeds L2 cache)", kb)
	} else if info.ExceedsL1 {
		info.Description = fmt.Sprintf("%.2f KB (exceeds L1 cache)", kb)
	} else {
		info.Description = fmt.Sprintf("%.2f KB (within L1 cache)", kb)
	}

	return info
}
