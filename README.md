<h2 align='center'>
  <img src="https://cdn.infinitybots.xyz/images/png/Infinity5.png" height='100px' width='100px' />
  <br> 
  Calypso
</h2>
<p align="center">
 Simple Tool for Migrating Data From Any Database to Postgres
</p>

<hr>

## You should be able to rawly plugin your Go structs to this tool

The ``bson`` and ``json`` struct tags define the db column name (``bson`` for historical reasons) and the postgres end name (json) respectively.

A primary key of ``itag`` is created to identify each element uniquely.

Place all structs to backup in ``schemas.go`` and then add them to ``backupSchemas`` function. Remove existing schemas if present.

### Extra options 

These extra options are placed in struct tags in your schema

- ``mark`` -> Marks a custom datatype to use
- ``default`` -> Sets a default when in doubt. A default value of ``SKIP`` skips the whole row when it is encountered.
- ``defaultfunc`` -> Sets a default func that *is* exported in exported functions. Exported functions can use all functions in pubfn.go
- ``log`` -> Whether to log or not
- ``tolist`` -> Whether or not to convert string element to a list of strings (if you're schema is bad)
- ``unique`` -> Whether or not a unique constaint should be set (``true`` or default ``false``)
- ``notnull`` -> Force not null to be set
- ``fkey`` -> The foreign key to set. Format is ``parent table name,column name``
- ``omitfield`` -> Whether or not to omit this field, a default value will be used in this case

### Daemon

For the purposes of logging and asking for user input while migrating, a foreground ``daemon`` is required/used. The daemon is written in python. Run ``cd daemon && python3 daemon.py`` to start it.

### Usage

1. Add your schemas and export functions (if you need custom code to be run before (``pre``) or as a default (``defaultfunc``))

**Example:**

```go
        UserID                    string         `bson:"userID" json:"user_id" unique:"true" default:"SKIP" pre:"usertrim"`
        Username                  string         `bson:"username" json:"username" defaultfunc:"getuser" default:"User"`
PackVotes                 map[string]any `bson:"pack_votes" json:"pack_votes" default:"{}"`
```
2. If you wish to add any migrations, add them to ``migrations/miglist.go``
3. Do ``go build`` to build the tool
4. Run ``hepatitis-antiviral`` 

## Sources

Some db sources are implemented by default:

- ``mongo`` -> MongoDB
- ``jsonfile`` -> JSON File

``postgres`` as a data source is only implemented as a ``backup`` source at this time. This means it can only be used with the WIP backup feature (seperate from the main features of this tool).