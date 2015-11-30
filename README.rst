=======================================
logias
=======================================

- logias is a simple server monitoring app.

Typical installation
---------------------------------------

::

    go get github.com/yuin/logias
    (create logias.lua)
    logias gen-sysvinit-script -c logias.lua > /etc/init.d/logias
    service logias start

Basic concepts
---------------------------------------

- Target : Target is a monitoring definition. There are 3 types of targets.
    1. ``target.FILE`` : Monitoring each line of the file.
    2. ``target.CMD`` :  Monitoring each line of command outputs.
    3. ``target.LUA`` :  Monitoring a lua function result table.
- State : State is a store that can be used for saving information about a single target.
- Parser : Parer parses stdouts of an external command or a lua function and converts it into a lua table.
- Parsed object : A lua table that was parsed by the Parser or returned by a lua function.
- Filter : Filter is a single action for each monitoring target element.
- Filter Group : Filter Group is a series of filters. The filter will only be run if precedent filter is acceptable.
- Notifier : Notifier is called by a filter and notify target state to users.

Monitoring flow
---------------------------------------

1. Call a ``init_state`` function to create a state object(a lua table object).
2. Get a monitoring element.
    - ``target.FILE`` : Read next line of the file.
    - ``target.CMD`` :  Execute the external command and apply a ``parser`` function to its stdout.
    - ``target.LUA`` : Call a ``fn`` lua function(the function must return a table object).
3. Evaluate the filter groups.
    1. Evaluate the filter.
    2. If the filter is acceptable, evaluate the next filter.
4. Wait ``interval`` seconds.

Configuration
---------------------------------------
Global settings
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
**stat_dir(string:directory path)**

logias writes ``target.FILE`` monitoring status to this directory.

**log_file(string:file path)**

A log file path

**log_level(enum: loglevel.(DEBUG|INFO|WARN|ERROR))**

A log level

**on_system_error:(function(string:error level, string:error message))**

If a system error occurs while logias is running, logias calls this function.

**downtime(function() bool:is in downtime) -> bool**

A Function that returns true when scheduled downtime, otherwise false.

Builtin downtime functions

- ``downtimefile(string: file)`` : Read a file that describes a downtime definition.The downtime defition format:

.. code-block::
    
    START:2001-02-03 04:00:00+09:00
    END:2001-02-03 10:00:00+09:00

In case of the argument is a file glob, A last one of a list that is sorted by name alphabetically is used for a downtime defitnition. If the argument is ``/tmp/downtime_*.txt`` and ``/tmp/downtime_00.txt`` and ``/tmp/downtime_01.txt`` exist, ``/tmp/downtime_01.txt`` will be used for a downtime definition.

Low level API: Builtin Filters
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

**match(table: regexp)**

A Filter that accepts if given line matches the ``regexp`` . Example:

.. code-block:: lua
    
    match {".*ERROR.*"}

**notmatch(table: regexp)**

A Filter that accepts if given line does not match the ``regexp`` . Example:

.. code-block:: lua
    
    match {".*Exception.*"}

**test(table: testfunc)**

A filter that accepts if the ``func`` returns ``true`` . The ``testfunc`` is called with these arguments: a state(table), a line(string) and a parsed object(maybe table). Example:

.. code-block:: lua
    
    test {function(state, line, obj) 
            return obj.value > 80
          end}

**action(table: func)**

A fiter that executes the ``func`` and always accepts. The ``func`` is called with these arguments: a state(table), a line(string) and a parsed object(maybe table). Example:

.. code-block:: lua
    
    action {function(state, line, obj) 
            log("INFO", "value: " .. tostring(obj.value))
          end}

**notify(table: {string: level, string: code, string: message})**

A filter that calls a notifier that was determied from the ``code`` or the ``level`` and never accepts. The ``code`` takes priority over the ``level`` . Example:

.. code-block:: lua
    
    notify {level="ERROR", message="CPU threshold exceeded"}
    notify {code="E0001"}



Low level API: Target settings
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

File monitoring
+++++++++++++++++++++++++

.. code-block:: lua

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

table key:string
    A file path.

type:enum(target.FILE)
    Inidicates this target is file monitoring.

Command monitoring
+++++++++++++++++++++++++

.. code-block:: lua

    ["sysinfo.sh"] = {
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
        }
      }
    }

table key:string
    A command line to execute.

type:enum(target.CMD)
    Inidicates this target is command monitoring.

parser:function(string: stdout) table
    A Function that receives the command output as a string, parse it into a table, and returns the table.

**Builtin parser**

- ``parseltsv`` : A parser for the ltsv format.

Lua function monitoring
+++++++++++++++++++++++++

.. code-block:: lua

    ["my-lua-monitoring"] = {
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

table key:string
    An identifier of this target.

type:enum(target.LUA)
    Inidicates this target is lua function monitoring.

fn:function() table
    A function that returns a table.

High level API: Service settings
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Service is a hight level API combining low level API functions.

.. code-block:: lua

    ["test.sh"] = service {
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

type:enum(target.FILE)
    A target type. This defaults to ``target.CMD`` . 

interval:number
    A monitoring interval in seconds. This defaults to ``60`` .

parser:function(string: stdout) table
    A Function that receives the command output as a string, parse it into a table, and returns the table. This defaults to ``parseltsv`` .

message:string
    A notification message template. In this template string, you can use these values : ``{{.name}}`` , ``{{.level}}``, ``{{.state}}`` , ``{{.op}}`` , ``{{.val}}`` , ``{{.count}}`` , ``{{.name_for_human}}`` , and ``{.notification_code}}`` . This defaults to ``"{{.name_for_human}} notification"``

attributes:table
    - table key : An attribute name of the parsed object.
    - name_for_human(string) : A human-readable name.
    - notification_code(string): A value will be used as a ``code`` parameter for the ``notify`` function.
    - thresholds(table) : A list of monitoring thresholds. A table key is a state one of the following string: "CRIT", "ERROR", "WARN" and "NORMAL" . A table value is a string that is separated by a space. First value is a comparison operator name(please refer to ``threshold`` function). Second value is a threshold value for the comparison operator. Third values is a ``count`` parameter for the ``threshold`` function.

Service stores following informations in ``state[attr_name]`` :

- name(string): A human-readable name.
- values(nqueue) : A series of attributes values. A last item is newer.
- current_state(string)
- previous_state(string)
- last_message(string)

And sets ``attribute_name`` of the parsed object to current attribute name. You can use these informations in a notifier like the following:

.. code-block:: lua

    E0001 = function(state, obj, message, level, code)
      local v = state[obj.attribute_name]
      for i, attr in ipairs({"previous_state", "current_state", "last_message"}) do
        print(attr .. ":" .. v[attr])
      end
      print("value:"..tostring(v.values:at(-1))) -- get latest value
      logias.notifiers.default(state, obj, message, level, code)
    end


Notifier settings
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Notifers are called in the following order:

1. By code: If the code is not blank, logias uses the code notifier.
2. By level: If the code is blank and the level is not blank, logias uses the level notifier.
3. Default notifier: If both the code and level are blank, logias uses the default notifier.

.. code-block:: lua

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

A notifier is a function:

`function(table:state, table:prased object, string:message, string:log level, string:code)`

Helper functions and classes
---------------------------------------

**log(string: level, string: message)**

Write given message to the ``log_file`` .

**template(string: template, table: values)**

Expand given ``template`` with the ``values`` . This function uses the ``text/template`` package.

**mail(table: attrs) -> (bool, [string])**

Send a email using a SMTP server. ``attrs`` has these keys: 

- ``user`` : SMTP user name.
- ``authhost`` : SMTP authorization host and port.
- ``host`` : SMTP host and port.
- ``from`` : From header value.
- ``to``   : To header value. This value can be a list of a string or a string.
- ``subject`` : Mail subject
- ``body`` : Mail body

This function returns ``true`` , or, in case of errors, ``false`` plus an error message. 

**isindowntime() -> bool**

Return ``true`` if the logias is in downtime, otherwise ``false`` .

**nqueue.new(number: size) -> nqueue**

``nqueue`` is a FIFO number value queue. ``size`` is a number that sets the upperbound limit on the number of items that can be placed in the queue.

**nqueue:put(number: value)**

Put the ``value`` into the queue.

**nqueue:at(number: index) -> number** 

Return Nth item of the queue, origin 1. Negative indices start counting from the end, with -1 being the last item.

**nqueue:pop() -> number** 

Remove and return an item from the last of the queue. If no items are present, returns a ``nil`` .

**nqueue:max() -> number** 

Return a maximum value in the the queue. If no items are present, returns a ``nil`` .

**nqueue:min() -> number** 

Return a minimum value in the the queue. If no items are present, returns a ``nil`` .

**#nqueue -> number**

Return the current number of items.

**threshold(table: attrs) -> function**

Creates a new function that can be use as the ``testfunc`` .

``name`` is a key name of a parsed object.  ``val`` is a threshold of the value.
``op`` is a comparison operator name. Thease operators are available: ``gt``, ``ge``, ``lt``, ``le``, ``ne``, ``eq``, ``range``. ``range`` takes a string like ``"80,90"`` and the others take a number.
As a result of the comparison of a current parsed object value and ``val``, if last ``count`` items exceed the threshold, the function returns ``true``, otherwise ``false`` .

Rotating the log_file
---------------------------------------
logias re-open the ``log_file`` when receiving a ``USR1`` signal.


License
----------------------------------------------------------------
MIT

Author
----------------------------------------------------------------
Yusuke Inuzuka
