package digitalocean

func convertEnvMapToList(env map[string]string) []string {
	envList := []string{}

	for key, value := range env {
		envList = append(envList, key+"="+value)
	}

	return envList
}

func removeDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}
