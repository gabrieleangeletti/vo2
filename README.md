# vo2

Fitness data import.

## Tools

* [Air](https://github.com/air-verse/air) (Hot reloading)
* [Migrate](https://github.com/golang-migrate/migrate) (DB migrations)

## TODO:

* Optimize storage of empty threshold analysis results. Preferred options: (1) separate status table, (2) nullable column in `activities_endurance` to indicate processed status.
* Improve time at LT1/LT2 algorithm based off of known activities.
