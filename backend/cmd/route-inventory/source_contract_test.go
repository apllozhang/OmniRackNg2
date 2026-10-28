package main

import "testing"

func TestSourceContractUsesActualBindingAndJSONVisibility(t *testing.T) {
	contract, err := readSourceContract("../../internal")
	if err != nil {
		t.Fatal(err)
	}
	schemas := contract["schemas"].(map[string]schema)
	login := schemas["handler_loginRequest"]
	props := login["properties"].(map[string]interface{})
	if props["username"].(schema)["maxLength"] != 100 {
		t.Fatalf("lost login validation: %#v", login)
	}
	if len(login["required"].([]string)) != 2 {
		t.Fatalf("lost required login fields: %#v", login)
	}
	room := schemas["handler_roomWriteRequest"]["properties"].(map[string]interface{})
	if room["racksPerRow"].(schema)["type"] != "integer" {
		t.Fatalf("custom optionalInt must be integer, got %#v", room["racksPerRow"])
	}
	if room["name"] == nil || room["version"] == nil {
		t.Fatal("embedded Room/BaseModel fields missing")
	}
	for _, name := range []string{"model_User", "model_LDAPConfigView", "service_NotificationView"} {
		p := schemas[name]["properties"].(map[string]interface{})
		for _, secret := range []string{"passwordHash", "bindPassword", "smtpPasswordCipher", "weComWebhookCipher"} {
			if _, exists := p[secret]; exists {
				t.Fatalf("%s exposes hidden %s", name, secret)
			}
		}
	}
	handlers := contract["handlers"].(map[string]interface{})
	updateUser := handlers["AdminHandler.UpdateUser"].(map[string]interface{})
	query := updateUser["query"].(map[string]schema)
	if query["version"]["x-required"] != true {
		t.Fatal("user update requires version in query")
	}
	disconnect := handlers["PDUHandler.Disconnect"].(map[string]interface{})
	if disconnect["requestSchema"] == nil {
		t.Fatal("DELETE pdu-connections requires a JSON version body")
	}
	deleteCatalog := handlers["AssetOperationsHandler.DeleteCatalog"].(map[string]interface{})
	if deleteCatalog["query"].(map[string]schema)["version"]["minimum"] != 1 {
		t.Fatal("closure factory version query missing")
	}
	if len(contract["handlerErrorDefinitions"].([]map[string]interface{})) == 0 {
		t.Fatal("missing error definitions")
	}
}
