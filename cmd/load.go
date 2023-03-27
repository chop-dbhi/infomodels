package cmd

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/infomodels/database"
	"github.com/infomodels/datadirectory"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"time"
)

var loadCmd = &cobra.Command{
	Use:   "load [flags] DATADIR",
	Short: "load a dataset into a database (and add indexes and constraints)",
	Long: `Load the dataset in DATADIR into the dburi specified database

Load the dataset in DATADIR by using the metadata to determine which
file to load into which table. The model tables are created, data
loaded, indexes created, and finally constraints added.

The tables are automatically vacuum/analyzed after they are loaded.

The required searchPath switch is a PostgreSQL search_path value
containing a comma-separated list of schema names. The first schema in
the list is the primary schema into which the data will be
loaded. Additional schemas may be required when applying constraints
if the loaded tables have foreign keys into other schemas.
`,
	Run: func(cmd *cobra.Command, args []string) {

		var (
			d   *datadirectory.DataDirectory
			cfg *datadirectory.Config
			db  *database.Database
			arg string
			err error
		)

		// Enforce single data directory argument.
		if len(args) != 1 {
			log.WithFields(log.Fields{
				"args": args,
			}).Fatal("load requires 1 argument")
		}

		arg = args[0]

		// Enforce required dburi.
		if viper.GetString("dburi") == "" {
			log.Fatal("load requires a dburi")
		}

		// Enforce required search path.
		if viper.GetString("searchPath") == "" {
			log.Fatal("load requires a searchPath")
		}

		log.WithFields(log.Fields{
			"directory": arg,
		}).Info("beginning dataset loading")

		// Make the DataDirectory object and load it from the metadata file,
		// see cmd/validate.go for that process.
		cfg = &datadirectory.Config{DataDirPath: arg}
		d, err = datadirectory.New(cfg)
		if err != nil {
			log.Fatal(fmt.Sprintf("Error reading data directory: %v", err))
		}

		err = d.ReadMetadataFromFile()
		if err != nil {
			log.Fatal(fmt.Sprintf("Error reading metadata file: %v", err))
		}

		// The data model in the data directory can be overridden by the
		// --model command line switch. E.g. the non-vocbulary `pedsnet`
		// data model is ordinarily overridden as `--model=pedsnet-core`,
		// and the vocabulary is overridden as `--model=pedsnet-vocab`

		dataModel := d.Model
		if viper.GetString("model") != "" {
			dataModel = viper.GetString("model")
		}

		modelVersion := d.ModelVersion
		if viper.GetString("modelv") != "" {
			modelVersion = viper.GetString("modelv")
		}

		excludeTables := ""
		if dataModel == "pcornet" {
			 excludeTables = "dummy"
		}

		// Open the Database object using information from the DataDirectory
		// object. Create tables and load data, then constraints and indexes.

		// TODO: should we enforce (in validate) that all models and model
		// versions in the metadata file are the same, and in
		// datadirectory, should ReadMetadataFromFile set the model and
		// model version in the datadirectory object? Eh, probably not.
		// We should loop over the metadata, opening the database for each
		// table we load.

		logFields := log.Fields{
			"DataModel":    dataModel,
			"ModelVersion": modelVersion,
			"DbUrl":        viper.GetString("dburi"),
			"SearchPath":   viper.GetString("searchPath"),
			"Service":      viper.GetString("service"),
		}
		db, err = database.Open(dataModel, modelVersion, viper.GetString("dburi"), viper.GetString("searchPath"), viper.GetString("dmsaservice"), "", excludeTables)
		if err != nil {
			logFields["err"] = err.Error()
			log.WithFields(logFields).Fatal("Database Open failed")
		}

		if !viper.GetBool("undo") {

			err = db.CreateTables("strict")
			if err != nil {
				logFields["err"] = err.Error()
				log.WithFields(logFields).Fatal("CreateTables() failed")
			}

			start := time.Now()

			err = db.Load(d)
			if err != nil {
				logFields["err"] = err.Error()
				log.WithFields(logFields).Fatal("Load() failed")
			}

			// TODO: add a switch to prevent adding indexes or constraints
			// TODO: add separate commands for adding indexes and constraints

			elapsed := time.Since(start)
			logFields["durationMinutes"] = elapsed.Minutes()
			log.WithFields(logFields).Info("Loaded. Beginning to add indexes.")
                        if !viper.GetBool("noidx") {
	         		indexesStart := time.Now()
				err = db.CreateIndexes("strict")
				if err != nil {
					logFields["err"] = err.Error()
					log.WithFields(logFields).Fatal("Error while adding indexes")
				}	

				elapsed = time.Since(indexesStart)
				logFields["durationMinutes"] = elapsed.Minutes()
				log.WithFields(logFields).Info("Indexes added.")
			}
			if !viper.GetBool("nofk") {
				constraintsStart := time.Now()
				log.WithFields(logFields).Info("Beginning to add constraints.")
				err = db.CreateConstraints("strict")
				if err != nil {
					logFields["err"] = err.Error()
					log.WithFields(logFields).Fatal("Error while adding constraints")
				}	

				elapsed = time.Since(constraintsStart)
				logFields["durationMinutes"] = elapsed.Minutes()
				log.WithFields(logFields).Info("Constraints added.")
			}

			elapsed = time.Since(start)
			logFields["durationMinutes"] = elapsed.Minutes()
			log.WithFields(logFields).Info("Load complete.")

		} else {

			// Drop constraints, indexes, and tables while ignoring 'does not exist' errors.

			err = db.DropConstraints("normal")
			if err != nil {
				logFields["err"] = err.Error()
				log.WithFields(logFields).Fatal("Unexpected error while dropping constraints")
			}

			err = db.DropIndexes("normal")
			if err != nil {
				logFields["err"] = err.Error()
				log.WithFields(logFields).Fatal("Unexpected error while dropping indexes")
			}

			err = db.DropTables("normal")
			if err != nil {
				logFields["err"] = err.Error()
				log.WithFields(logFields).Fatal("Unexpected error while dropping tables")
			}

		}

		// TODO: figure out password handling that does not involve entering
		// it into a command line.
		// TODO: make this command idempotent so that it can wipe any existing
		// data and load the new stuff, to resume from a failure.

	},
}

func init() {

	// Register this command under the top-level CLI command.
	RootCmd.AddCommand(loadCmd)

	// Made these into global flags because I couldn't figure out how to use them in another subcommand also.
	// // Set up the load-command-specific flags.
	// loadCmd.Flags().StringP("dburi", "d", "", "Database URI to load the dataset into. Required.")
	// //	loadCmd.Flags().StringP("dbpass", "p", "", "Database password.")
	// loadCmd.Flags().StringP("searchPath", "s", "", "SearchPath for the load (secondary schemas may be needed for adding constraints). Required.")
	// loadCmd.Flags().Bool("undo", false, "Undo the load; delete all tables.")

	// // Bind viper keys to the flag values.
	// viper.BindPFlag("dburi", loadCmd.Flags().Lookup("dburi"))
	// //	viper.BindPFlag("dbpass", loadCmd.Flags().Lookup("dbpass"))
	// viper.BindPFlag("searchPath", loadCmd.Flags().Lookup("searchPath"))
	// viper.BindPFlag("undo", loadCmd.Flags().Lookup("undo"))
}
