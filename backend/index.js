import express from "express";

const app = express();
app.use(express.json());

app.get("/health", (_, res) => {
  res.json({ status: "ok", time: new Date() });
});

app.listen(3000, () => {
  console.log("Backend running on 3000");
});
