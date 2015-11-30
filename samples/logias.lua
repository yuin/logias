logias = {
  stat_dir = "/tmp",
  log_file = "/tmp/logias.log",
  log_level = loglevel.INFO,

  on_system_error = log,
  downtime = downtimefile([[/tmp/downtime*.txt]]),

  targets = {
    ["/tmp/server.log"] = {
      type     = target.FILE,
      interval = 3,
      initial_state = function()
        return {}
      end,
      filter_groups = {
        {
          match {".*ERROR.*"},
          notmatch {".*EX.*"},
          notify {level="ERROR", code="E0001"}
        }
      }
    },
    ["./test.sh"] = {
      type     = target.CMD,
      interval = 3,
      initial_state = function()
        return {}
      end,
      parser = parseltsv,
      filter_groups = {
        {
          test {threshold{name="cpu", op="ge", val=90, count=1}},
          notify {level="ERROR", code="E0001", message="CPU Threshold Exceeded.(>90%)"},
        },
        {
          test {threshold{name="cpu", op="range", val="80,90", count=3}},
          notify {level="WARN", message="CPU Threshold Exceeded.(>80%)"},
        },
        {
          test {threshold{name="cpu", op="le", val="80", count=3, recover=true}},
          notify {level="INFO", message="CPU recovered to the normal range."},
        },
        {
          action {
              function(state, line, obj)
                  print(#state.cpu, state.cpu:at(-1))
                  log("INFO", tostring(#state.cpu) .. " " .. tostring(state.cpu:at(-1)))
              end
          }
        }
      }
    },
    ["test.sh"] = service {
      interval = 3,
      attributes = {
        cpu = {
          name_for_human = "CPU",
          notification_code = "E0001",
          thresholds = {
            ERROR = "ge 90",
            WARN  = "range 80,90",
            NORMAL  = "le 80"
          }
        }
      }
    },
    ["mylua"] = {
      type = target.LUA,
      interval = 3,
      initial_state = function()
        return {}
      end,
      fn = function() 
        return {cpu = 89}
      end,
      filter_groups = {
        {
          test {threshold{name="cpu", op="ge", val=90, count=1}},
          notify {level="ERROR", code="E0001", message="CPU Threshold Exceeded.(>=90%)"}
        }
      }
    }
  },
  notifiers = {
    default = function(state, obj, message, level, code)
        log(level, template([[level: {{.level}} code: {{.code}} {{.message}}]], {level=level, code = code, message= message}))
    end,
    level = {
      CRIT = function(...)
        logias.notifiers.default(...)
      end,
      ERROR = function(...)
        logias.notifiers.default(...)
      end,
      WARN = function(...)
        logias.notifiers.default(...)
      end,
      INFO = function(...)
        logias.notifiers.default(...)
      end
    },
    code = {
      E0001 = function(...)
        logias.notifiers.default(...)
      end
    }
  }
}
