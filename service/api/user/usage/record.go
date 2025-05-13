package usage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"json_trans_api/config"
	"json_trans_api/pkg/httpclient"
	"log"
	"net/http"
	"time"
)

// RecordUsage 记录用户的使用量
func RecordUsage(userID string, characterCount int, requestType string) error {
	baseURL := fmt.Sprintf("%s/rest/v1/usage_records", config.Cfg.Supabase.SupabaseUrl)
	
	usageData := map[string]interface{}{
		"user_id":         userID,
		"character_count": characterCount,
		"request_type":    requestType,
		"created_at":      time.Now(),
	}
	
	usageJSON, err := json.Marshal(usageData)
	if err != nil {
		return err
	}
	
	req, err := http.NewRequest("POST", baseURL, bytes.NewBuffer(usageJSON))
	if err != nil {
		return err
	}
	
	req.Header.Set("apikey", config.Cfg.Supabase.SupabaseSecretKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Cfg.Supabase.SupabaseSecretKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")
	
	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("记录使用量失败，状态码: %d", resp.StatusCode)
	}
	
	return nil
}

// RecordUsageAsync 异步记录用户的使用量
func RecordUsageAsync(userID string, characterCount int, requestType string) {
	go func() {
		err := RecordUsage(userID, characterCount, requestType)
		if err != nil {
			log.Printf("异步记录使用量失败: %v", err)
		}
	}()
}