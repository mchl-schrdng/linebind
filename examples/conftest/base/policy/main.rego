package main

deny contains message if {
  container := input.spec.containers[_]
  container.securityContext.privileged == true
  message := sprintf("Container %s must not be privileged", [container.name])
}
