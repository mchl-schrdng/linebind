package main

valid_pod if {
  input.apiVersion == "v1"
  input.kind == "Pod"
  is_object(input.spec)
  is_array(input.spec.containers)
  count(input.spec.containers) > 0
}

deny contains "Expected a v1 Pod with a nonempty spec.containers array" if {
  not valid_pod
}

deny contains message if {
  some group in {"containers", "initContainers", "ephemeralContainers"}
  not is_array(object.get(input.spec, group, []))
  message := sprintf("spec.%s must be an array when present", [group])
}

unprivileged(container) if {
  is_object(container)
  context := object.get(container, "securityContext", {})
  is_object(context)
  object.get(context, "privileged", false) == false
}

deny contains message if {
  some group in {"containers", "initContainers", "ephemeralContainers"}
  some index, container in input.spec[group]
  not unprivileged(container)
  message := sprintf("spec.%s[%v] must omit privileged or set it to boolean false", [group, index])
}
