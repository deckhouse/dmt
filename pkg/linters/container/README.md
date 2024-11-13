Checks containers inside the template spec. This linter protects against the next cases:
 - containers with the duplicated names
 - containers with the duplicated env variables
 - misconfigured images repository and digest
 - imagePullPolicy is "Always" (should be unspecified or "IfNotPresent")
 - ephemeral storage is not defined in .resources
 - SecurityContext is not defined
 - container uses port <= 1024