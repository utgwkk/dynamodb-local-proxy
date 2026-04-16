"com.amazonaws.dynamodb.v20120810#UnknownOperationException" as $full_exception_type
| "Tagging is not currently supported in DynamoDB Local." as $message
| if .__type != $full_exception_type then .__type = $full_exception_type end
| if .message != $message then .message = $message end
