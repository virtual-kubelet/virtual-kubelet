package config

var schemaV1 = `{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "id": "config_schema_v1.json",

  "type": "object",

  "patternProperties": {
    "^[a-zA-Z0-9._-]+$": {
      "$ref": "#/definitions/service"
    }
  },

  "additionalProperties": false,

  "definitions": {
    "service": {
      "id": "#/definitions/service",
      "type": "object",

      "properties": {
        "cap_add": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "cap_drop": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "cgroup_parent": {"type": "string"},
        "command": {
          "oneOf": [
            {"type": "string"},
            {"type": "array", "items": {"type": "string"}}
          ]
        },
        "container_name": {"type": "string"},
        "cpu_shares": {"type": ["number", "string"]},
        "cpu_quota": {"type": ["number", "string"]},
        "cpuset": {"type": "string"},
        "dns": {"$ref": "#/definitions/string_or_list"},
        "dns_search": {"$ref": "#/definitions/string_or_list"},
        "domainname": {"type": "string"},
        "entrypoint": {
          "oneOf": [
            {"type": "string"},
            {"type": "array", "items": {"type": "string"}}
          ]
        },
        "env_file": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "environment": {"$ref": "#/definitions/list_or_dict"},

        "extends": {
          "oneOf": [
            {
              "type": "string"
            },
            {
              "type": "object",

              "properties": {
                "service": {"type": "string"},
                "file": {"type": "string"}
              },
              "required": ["service"],
              "additionalProperties": false
            }
          ]
        },

        "expose": {
          "type": "array",
          "items": {
            "type": ["string", "number"]
          },
          "uniqueItems": true
        },

        "external_links": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "hostname": {"type": "string"},
        "image": {"type": "string"},
        "labels": {"$ref": "#/definitions/list_or_dict"},
        "links": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "mem_limit": {"type": ["number", "string"]},
        "memswap_limit": {"type": ["number", "string"]},
        "noauto_volume": {"type": "boolean"},

        "ports": {
          "type": "array",
          "items": {
            "type": ["string", "number"]
          },
          "uniqueItems": true
        },

        "stop_signal": {"type": "string"},
        "security_groups": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "restart": {"type": "string"},
        "stdin_open": {"type": "boolean"},
        "tty": {"type": "boolean"},
        "volumes": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "working_dir": {"type": "string"},
        "user": {"type": "string"},

        "size": {"type": "string"},
        "fip": {"type": "string"}
      },

      "dependencies": {
      },
      "additionalProperties": false
    },

    "string_or_list": {
      "oneOf": [
        {"type": "string"},
        {"$ref": "#/definitions/list_of_strings"}
      ]
    },

    "list_of_strings": {
      "type": "array",
      "items": {"type": "string"},
      "uniqueItems": true
    },

    "list_or_dict": {
      "oneOf": [
        {
          "type": "object",
          "patternProperties": {
            ".+": {
              "type": ["string", "number", "null"]
            }
          },
          "additionalProperties": false
        },
        {"type": "array", "items": {"type": "string"}, "uniqueItems": true}
      ]
    },

    "constraints": {
      "service": {
        "id": "#/definitions/constraints/service",
        "anyOf": [
          {
            "required": ["image"]
          }
        ]
      }
    }
  }
}
`

var schemaV2 = `{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "id": "config_schema_v2.0.json",
  "type": "object",

  "patternProperties": {
    "^[a-zA-Z0-9._-]+$": {
      "$ref": "#/definitions/service"
    }
  },

  "additionalProperties": false,

  "definitions": {
    "service": {
      "id": "#/definitions/service",
      "type": "object",
      "properties": {
        "cap_add": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "cap_drop": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "cgroup_parent": {"type": "string"},
        "command": {
          "oneOf": [
            {"type": "string"},
            {"type": "array", "items": {"type": "string"}}
          ]
        },
        "container_name": {"type": "string"},
        "cpu_shares": {"type": ["number", "string"]},
        "cpu_quota": {"type": ["number", "string"]},
        "cpuset": {"type": "string"},
        "depends_on": {"$ref": "#/definitions/list_of_strings"},
        "dns": {"$ref": "#/definitions/string_or_list"},
        "dns_search": {"$ref": "#/definitions/string_or_list"},
        "domainname": {"type": "string"},
        "entrypoint": {
          "oneOf": [
            {"type": "string"},
            {"type": "array", "items": {"type": "string"}}
          ]
        },
        "env_file": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "environment": {"$ref": "#/definitions/list_or_dict"},
        "extends": {
          "oneOf": [
            {
              "type": "string"
            },
            {
              "type": "object",

              "properties": {
                "service": {"type": "string"},
                "file": {"type": "string"}
              },
              "required": ["service"],
              "additionalProperties": false
            }
          ]
        },

        "expose": {
          "type": "array",
          "items": {
            "type": ["string", "number"]
          },
          "uniqueItems": true
        },

        "external_links": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "hostname": {"type": "string"},
        "image": {"type": "string"},
        "labels": {"$ref": "#/definitions/list_or_dict"},
        "links": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "mem_limit": {"type": ["number", "string"]},
        "memswap_limit": {"type": ["number", "string"]},
        "network_mode": {"type": "string"},
        "noauto_volume": {"type": "boolean"},

        "networks": {
          "oneOf": [
            {"$ref": "#/definitions/list_of_strings"},
            {
              "type": "object",
              "patternProperties": {
                "^[a-zA-Z0-9._-]+$": {
                  "oneOf": [
                    {
                      "type": "object",
                      "properties": {
                        "aliases": {"$ref": "#/definitions/list_of_strings"},
                        "ipv4_address": {"type": "string"},
                        "ipv6_address": {"type": "string"}
                      },
                      "additionalProperties": false
                    },
                    {"type": "null"}
                  ]
                }
              },
              "additionalProperties": false
            }
          ]
        },

        "ports": {
          "type": "array",
          "items": {
            "type": ["string", "number"]
          },
          "uniqueItems": true
        },

        "restart": {"type": "string"},
        "stdin_open": {"type": "boolean"},
        "stop_signal": {"type": "string"},
        "security_groups": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "tty": {"type": "boolean"},
        "user": {"type": "string"},
        "volumes": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "working_dir": {"type": "string"},

        "size": {"type": "string"},
        "fip": {"type": "string"}
      },

      "additionalProperties": false
    },

    "volume": {
      "id": "#/definitions/volume",
      "type": ["object", "null"],
      "properties": {
        "driver_opts": {
          "type": "object",
          "patternProperties": {
            "^.+$": {"type": ["string", "number"]}
          }
        },
        "external": {
          "type": ["boolean", "object"],
          "properties": {
            "name": {"type": "string"}
          }
        },
        "additionalProperties": false
      },
      "additionalProperties": false
    },

    "string_or_list": {
      "oneOf": [
        {"type": "string"},
        {"$ref": "#/definitions/list_of_strings"}
      ]
    },

    "list_of_strings": {
      "type": "array",
      "items": {"type": "string"},
      "uniqueItems": true
    },

    "list_or_dict": {
      "oneOf": [
        {
          "type": "object",
          "patternProperties": {
            ".+": {
              "type": ["string", "number", "null"]
            }
          },
          "additionalProperties": false
        },
        {"type": "array", "items": {"type": "string"}, "uniqueItems": true}
      ]
    },

    "constraints": {
      "service": {
        "id": "#/definitions/constraints/service",
        "anyOf": [
          {"required": ["image"]}
        ]
      }
    }
  }
}
`
