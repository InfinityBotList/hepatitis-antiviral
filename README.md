# Move from mongo to postgres migration tool

## You should be able to rawly plugin your Go structs to this tool

The ``bson`` and ``json`` struct tags define the mongoDB column name (bson) and the postgres end name (json) respectively.

A primary key of ``itag`` is created to identify each element uniquely.

Place all structs to backup in ``schemas.go`` and then add them to ``backupSchemas`` function. Remove old schemas if present

### Extra options 

These extra options are placed in struct tags in your schema

- ``mark`` -> Marks a custom datatype to use
- ``default`` -> Sets a default when in doubt
- ``defaultfunc`` -> Sets a default func that *is* exported in exported functions
- ``log`` -> Whether to log or not
- ``tolist`` -> Whether or not to convert string element to a list of strings (if you're schema is bad)
- ``unique`` -> Whether or not a unique constaint should be set (``true`` or default ``false``)
- ``notnull`` -> Force not null to be set
- ``fkey`` -> The foreign key to set. Format is ``parent table name,column name``
- ``omitfield`` -> Whether or not to omit this field, a default value will be used in this case

### Daemon

For the purposes of logging and asking for user input while migrating, a foreground ``daemon`` is required/used. The daemon is written in python. Run ``cd daemon && python3 daemon.py`` to start it