package main

import (
	"app/helper"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/joho/godotenv"
)

type RequestBody struct {
	AWSRegion string `json:"aws_region"`
}

func main() {
	http.HandleFunc("/monitor/servers", monitorServersHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func monitorServersHandler(w http.ResponseWriter, r *http.Request) {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file: %s", err)
	}
	AwsAccessKey := os.Getenv("AWS_ACCESS_KEY")
	AwsSecretKey := os.Getenv("AWS_SECRET_KEY")

	var requestBody RequestBody

	// Decode the request body into the requestBody struct
	errBody := json.NewDecoder(r.Body).Decode(&requestBody)
	if errBody != nil {
		helper.ErrorResponse(w, http.StatusBadRequest, "Failed to decode request body")
		return
	}

	// Ensure that AWS region is provided in the request body
	if requestBody.AWSRegion == "" {
		helper.ErrorResponse(w, http.StatusBadRequest, "Missing AWS region in request body")
		return
	}

	// Use the AWS region from the request body
	awsRegion := requestBody.AWSRegion

	creds := credentials.NewStaticCredentials(AwsAccessKey, AwsSecretKey, "")
	awsConfig := aws.NewConfig().WithCredentials(creds)

	sess, err := session.NewSession(awsConfig, &aws.Config{
		Region: aws.String(awsRegion), // Change to your desired region
	})
	if err != nil {
		helper.ErrorResponse(w, http.StatusInternalServerError, "Failed to create AWS session")
		return
	}
	ec2Svc := ec2.New(sess)

	// Retrieve information about your EC2 instances
	instances, err := getEC2Instances(ec2Svc)
	if err != nil {
		log.Fatal(err)
		helper.ErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve EC2 instances")
		return
	}

	monitoringData := generateMonitoringData(instances, sess)

	// Convert monitoring data to JSON
	monitoringJSON, err := json.Marshal(monitoringData)
	if err != nil {
		helper.ErrorResponse(w, http.StatusInternalServerError, "ailed to marshal monitoring data")
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Write monitoring data JSON to the response
	w.Write(monitoringJSON)

}

func getEC2Instances(ec2Svc *ec2.EC2) ([]*ec2.Instance, error) {
	// Describe instances to get information about all instances
	result, err := ec2Svc.DescribeInstances(nil)
	if err != nil {
		return nil, err
	}

	var instances []*ec2.Instance
	for _, reservation := range result.Reservations {
		instances = append(instances, reservation.Instances...)
	}

	return instances, nil
}

func generateMonitoringData(instances []*ec2.Instance, session *session.Session) map[string]interface{} {
	monitoringData := make(map[string]interface{})
	cloudwatchSvc := cloudwatch.New(session)

	for _, instance := range instances {
		instanceID := *instance.InstanceId
		cpuUtilization, err := getEC2CPUUtilization(cloudwatchSvc, instanceID)
		if err != nil {
			log.Printf("Failed to retrieve CPU utilization for instance %s: %v", instanceID, err)
			continue
		}
		instanceType := *instance.InstanceType
		instanceState := *instance.State
		keyName := *instance.KeyName
		cpuOptions := *instance.CpuOptions
		architecture := *instance.Architecture
		serverTags := *instance.Tags[0].Value
		publicDomain := *instance.PublicDnsName
		// Add more monitoring metrics as needed

		monitoringData[instanceID] = map[string]interface{}{
			"instance_type":   instanceType,
			"key_name":        keyName,
			"architecture":    architecture,
			"instance_state":  instanceState,
			"cpu_options":     cpuOptions,
			"server_tags":     serverTags,
			"public_dns":      publicDomain,
			"cpu_utilization": cpuUtilization,

			// Add more metrics here
		}
	}
	return monitoringData
}

func getEC2CPUUtilization(ec2Svc *cloudwatch.CloudWatch, instanceID string) ([]float64, error) {
	currentTime := time.Now()
	fiveMinutesAgo := currentTime.Add(-5 * time.Minute)

	input := &cloudwatch.GetMetricDataInput{
		StartTime: &fiveMinutesAgo,
		EndTime:   &currentTime,
		MetricDataQueries: []*cloudwatch.MetricDataQuery{
			{
				Id: aws.String("cpuUtilization"),
				MetricStat: &cloudwatch.MetricStat{
					Metric: &cloudwatch.Metric{
						Namespace:  aws.String("AWS/EC2"),
						MetricName: aws.String("CPUUtilization"),
						Dimensions: []*cloudwatch.Dimension{
							{
								Name:  aws.String("InstanceId"),
								Value: aws.String(instanceID),
							},
						},
					},
					Period: aws.Int64(60), // 1 minute intervals
					Stat:   aws.String("Average"),
				},
				ReturnData: aws.Bool(true),
			},
		},
	}

	result, err := ec2Svc.GetMetricData(input)
	if err != nil {
		return nil, err
	}

	dataMetric := *result.MetricDataResults[0]

	floatArray := make([]float64, len(dataMetric.Values))
	for i, ptr := range dataMetric.Values {
		floatArray[i] = *ptr
	}

	if len(result.MetricDataResults) > 0 {
		return floatArray, nil
	}

	return nil, nil
}
