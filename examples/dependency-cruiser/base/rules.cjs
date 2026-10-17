module.exports = {
  allowedSeverity: "error",
  allowed: [
    { from: { path: "^src/client/" }, to: { path: "^src/(client|api|shared)/" } },
    { from: { path: "^src/api/" }, to: { path: "^src/(api|domain|adapters|shared)/" } },
    { from: { path: "^src/domain/" }, to: { path: "^src/(domain|shared)/" } },
    { from: { path: "^src/adapters/" }, to: { path: "^src/(adapters|domain|shared)/" } },
    { from: { path: "^src/adapters/" }, to: { dependencyTypes: ["core"] } },
    { from: { path: "^src/shared/" }, to: { path: "^src/shared/" } },
  ],
  forbidden: [
    {
      name: "no-cycles",
      severity: "error",
      from: {},
      to: { circular: true },
    },
    {
      name: "unmapped-source",
      severity: "error",
      from: {
        path: "^src/",
        pathNot: "^src/(client|api|domain|adapters|shared)/",
        orphan: true,
      },
      to: {},
    },
  ],
};
