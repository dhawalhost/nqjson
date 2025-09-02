package njson

import (
	"fmt"
)

func ExampleSet_simple() {
	json := []byte(`{
		"name": "John Doe",
		"age": 30,
		"address": {
			"street": "123 Main St",
			"city": "New York"
		}
	}`)

	// Simple value replacement - uses fast path (no compilation)
	result, err := Set(json, "age", 31)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Add a new field
	result, err = Set(result, "email", "john.doe@example.com")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Nested field update
	result, err = Set(result, "address.city", "Boston")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Display the updated JSON
	fmt.Println(string(result))

	// Output:
	// {
	//   "address": {
	//     "city": "Boston",
	//     "street": "123 Main St"
	//   },
	//   "age": 31,
	//   "email": "john.doe@example.com",
	//   "name": "John Doe"
	// }
}

func ExampleSet_array() {
	json := []byte(`{
		"users": [
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"}
		]
	}`)

	// Update array element
	result, err := Set(json, "users.1.name", "Robert")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Add new array element
	newUser := map[string]interface{}{
		"id":   3,
		"name": "Charlie",
	}
	result, err = Set(result, "users.2", newUser)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Display the updated JSON
	fmt.Println(string(result))

	// Output:
	// {
	//   "users": [
	//     {
	//       "id": 1,
	//       "name": "Alice"
	//     },
	//     {
	//       "id": 2,
	//       "name": "Robert"
	//     },
	//     {
	//       "id": 3,
	//       "name": "Charlie"
	//     }
	//   ]
	// }
}

func ExampleSetWithOptions() {
	json := []byte(`{
		"settings": {
			"theme": "dark",
			"notifications": true
		},
		"data": [1, 2, 3]
	}`)

	// Merge objects instead of replacing
	newSettings := map[string]interface{}{
		"fontSize": 14,
		"language": "en-US",
	}

	options := SetOptions{
		MergeObjects: true,
	}

	result, err := SetWithOptions(json, "settings", newSettings, &options)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Merge arrays
	options.MergeArrays = true
	result, err = SetWithOptions(result, "data", []interface{}{4, 5}, &options)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Display the updated JSON
	fmt.Println(string(result))

	// Output:
	// {
	//   "data": [
	//     1,
	//     2,
	//     3,
	//     4,
	//     5
	//   ],
	//   "settings": {
	//     "fontSize": 14,
	//     "language": "en-US",
	//     "notifications": true,
	//     "theme": "dark"
	//   }
	// }
}

func ExampleCompileSetPath() {
	json := []byte(`{"users":[{"name":"Alice","role":"admin"},{"name":"Bob","role":"user"}]}`)

	// For repeated operations, compile the path once
	path, err := CompileSetPath("users.0.role")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Use the compiled path multiple times
	result, err := SetWithCompiledPath(json, path, "super-admin", nil)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Compiled paths are especially useful in loops
	userNames := []string{"Alice", "Bob", "Charlie"}
	userRoles := []string{"owner", "editor", "viewer"}

	// Add each user (in real code, this would be in a loop)
	for i, name := range userNames[:1] { // Just use first user for example
		// Compile path once
		userPath, _ := CompileSetPath(fmt.Sprintf("users.%d.name", i))
		rolePath, _ := CompileSetPath(fmt.Sprintf("users.%d.role", i))

		// Use compiled path for repeated operations
		result, _ = SetWithCompiledPath(result, userPath, name, nil)
		result, _ = SetWithCompiledPath(result, rolePath, userRoles[i], nil)
	}

	// Display the updated JSON
	fmt.Println(string(result))

	// Output:
	// {
	//   "users": [
	//     {
	//       "name": "Alice",
	//       "role": "owner"
	//     },
	//     {
	//       "name": "Bob",
	//       "role": "user"
	//     }
	//   ]
	// }
}

func ExampleDelete() {
	json := []byte(`{
		"user": {
			"name": "John",
			"email": "john@example.com",
			"settings": {
				"theme": "dark",
				"notifications": true,
				"fontSize": 14
			}
		}
	}`)

	// Delete a nested field
	result, err := Delete(json, "user.settings.notifications")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Delete an entire object
	result, err = Delete(result, "user.settings")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Display the updated JSON
	fmt.Println(string(result))

	// Output:
	// {
	//   "user": {
	//     "email": "john@example.com",
	//     "name": "John"
	//   }
	// }
}
