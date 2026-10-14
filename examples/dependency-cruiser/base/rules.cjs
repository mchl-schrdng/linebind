module.exports = {
  forbidden: [
    {
      name: "client-must-use-api",
      severity: "error",
      from: { path: "^src/client/" },
      to: { path: "^src/domain/" },
    },
  ],
};
