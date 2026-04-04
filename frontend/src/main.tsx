import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { ConfigProvider, theme } from "antd";
import "antd/dist/reset.css";
import "./index.css";
import App from "./App";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ConfigProvider
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          colorPrimary: "#24413d",
          colorInfo: "#24413d",
          colorSuccess: "#2d6a4f",
          colorWarning: "#a85b3f",
          colorBgBase: "#f6f1e7",
          colorTextBase: "#10211f",
          borderRadius: 18,
          fontFamily: "'Manrope', 'Segoe UI', sans-serif",
        },
      }}
    >
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </ConfigProvider>
  </React.StrictMode>,
);
