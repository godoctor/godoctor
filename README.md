The "go doctor"

1. Description
2. Authors
3. Driver
  3.1 Calling
  3.2 Output
  3.3 JSON


## 3. Driver

### 3.1 Calling
  The "go doctor" can be called correctly using a command of the form
  `go-doctor [<flag> ...] <file> <refactoring> [<arg> ...]`

  Where <flag> (shown with defaults) may be a combination of:
    -h=false, show usage for go-doctor
    -format=plain, show output in 'plain' or 'json', see 3.2
    -pos="", specify a position of the format: (all ints) startLine,startCol:startLine,startCol
    -d=false, return diff of each file affected by given refactoring, see 3.2
    -p=false, return description of parameters for given refactoring, see 3.2
    -w=false, write refactored files instead of printing output
    -e=false, write results even if errors exist in log
    -scope="", give a scope (file or package) to do refactoring on

  <file> may be relative or absolute path

  <refactoring> may be one of
    null
    rename
    shortassign

  <arg> ... are all of the required params for a given <refactoring>,
  to see what these are first execute with -p for said <refactoring>.

### 3.2 Output

Output will be to stdout with the specified format.
The default is `format=plain`.
To get a more parsable output, specify `format=json`, see 3.3.
Each example below will specify an example "query" to get said output

#### Default output (-format=plain)

##### Params (-p)
  Example query: `go-doctor -p rename`

  Will output a new line separated description of the
  required parameters that are necessary to execute the given <refactoring>

##### Diff (-d)
  Example query: `go-doctor -d -pos=11,6:11,6 myFile.go rename newName`

  Will output diffs for each file affected, where
  each diff will be prefaced with the name of the file that was diffed
  followed by a blank line, and separated by another blank line
  directly after the diff. 
  
  WARNING: The file given next to +++ is only
  used for computing the diff, and will be removed directly after
  computation, do not attempt to patch this file. 

  Example:
  affectedFile.go

  --- affectedFile.go
  +++ ...

  + changes
  - changes

  nextFile.go

  ...

##### Changed Files (default)
  Example query: `go-doctor -pos=11,6:11,6 myFile.go rename newName`

  Will output the updated file for each file affected,
  where each file will be prefaced with the name of the file that
  was affected, followed by a blank line, and separated by another
  blank line directly after the file contents.

  Example:
  updatedFile.go

  import "fmt"

  func main() {
    int refactoredInt = 3
    fmt.Printf("%d", refactoredInt)
  }

  nextUpdatedFile.go

  ...

### 3.3 JSON protocol
  
  Here lies the json output for the go doctor, given on stdout.
  Each query below will provide a sample query and output.
  Descriptions for output will be provided below the given form.

##### params, from -p

  Example: `go-doctor -p rename`

  Output of the form:
  {
    "params": [
      param, ...
    ]
  }

  Where:

  "params" is an array of 'param', in the exact order required
  to be given to a later query (in <arg>). This may be empty, in
  which case there are no required parameters for the given refactoring.

  param is a string with a description of the required
  parameter necessary to execute a given refactoring.

##### diff, from -d

  Example: `go-doctor -d -pos=11,6:11,6 file.go rename newName`

  Output of the form:
  {
    "name": name,
    "log": {
      "entries": [
      {
        "severity": severity,
        "message": message,
        "filename": filename,
        "position": { 
          "offset": offset,
          "length": length
        }
      }]
    },
    "changes": {
      "filename": "diff"
    }
  }

  name is the string name of the refactoring for these changes

  "log" is an object of "entries".

  "entries" is an array of objects. This should be checked for
  emptiness before actually applying any changes.

  Each entry consists of:
    severity is a string (TODO int?) that is either:
      INFO, WARNING, ERROR or FATAL_ERROR

    message is a string description of the error/warning

    filename is a string filename where the error/warning occurred

    position is an object of:

      offset is an integer zero byte offset of where the error occurred in file
      length is an integer non-zero length relative to offset for error

  "changes" is an object containing all of the files affected by the
  refactoring along with a diff of the file and the updated file in the form:

    filename is a string of the affected file's path
    diff is the string contents of the diff file


##### Updated files (default output)

  Example: `go-doctor -pos=11,6:11,6 file.go rename newName`

  Output of the form:
  {
    "name": name,
    "log": {
      "entries": [
      {
        "severity": severity,
        "message": message,
        "filename": filename,
        "position": { 
          "offset": offset,
          "length": length
        }
      }]
    },
    "changes": {
      "filename": "contents"
    }
  }

  name is the string name of the refactoring for these changes

  "log" is an object of "entries".

  "entries" is an array of objects. This should be checked for
  emptiness before actually applying any changes.

  Each entry consists of:
    severity is a string (TODO int?) that is either:
      INFO, WARNING, ERROR or FATAL_ERROR

    message is a string description of the error/warning

    filename is a string filename where the error/warning occurred

    position is an object of:

      offset is an integer zero byte offset of where the error occurred in file
      length is an integer non-zero length relative to offset for error

  "changes" is an object containing all of the files affected by the
  refactoring along with a diff of the file and the updated file in the form:

    filename is a string of the affected file's path
    contents is the string contents of the updated file
