# Move from mongo to postgres migration tool

## You should be able to rawly plugin your Go structs to this tool

The ``bson`` and ``json`` struct tags define the mongoDB column name (bson) and the postgres end name (json) respectively.

A primary key of ``itag`` is created to identify each element uniquely.

Place all structs to backup in ``schemas.go`` and then add them to ``backupSchemas`` function. Remove old schemas if present

### Extra options

- ``mark`` -> Marks a custom datatype to use
- ``default`` -> Sets a default when in doubt
- ``log`` -> Whether to log or not
- ``tolist`` -> Whether or not to convert string element to a list of strings (if you're schema is bad)
- ``schema`` -> Use this when you need to define a custom type and/or constraints for your postgres column (AKA ``schema:"text unique not null"``)