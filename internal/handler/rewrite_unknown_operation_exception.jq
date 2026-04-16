"com.amazonaws.dynamodb.v20120810#UnknownOperationException" as $full_exception_type
| if .__type != $full_exception_type then .__type = $full_exception_type end
