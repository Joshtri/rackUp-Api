use super::AppError;

/// Trims and collapses inner whitespace: "  Budi   Santoso " → "Budi Santoso".
/// Same rule as `cleanName` in the frontend.
pub fn clean_name(name: &str) -> String {
    name.split_whitespace().collect::<Vec<_>>().join(" ")
}

/// Case-insensitive name comparison, like `sameName` in the frontend.
pub fn same_name(a: &str, b: &str) -> bool {
    a.to_lowercase() == b.to_lowercase()
}

/// Checks a cleaned, required text field's length in characters.
pub fn require_text(field: &str, value: &str, max_chars: usize) -> Result<(), AppError> {
    if value.is_empty() {
        return Err(AppError::Validation(format!("{field} is required")));
    }
    if value.chars().count() > max_chars {
        return Err(AppError::Validation(format!(
            "{field} must be at most {max_chars} characters"
        )));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cleans_whitespace() {
        assert_eq!(clean_name("  Budi \t  Santoso "), "Budi Santoso");
        assert_eq!(clean_name("   "), "");
    }

    #[test]
    fn compares_names_ignoring_case() {
        assert!(same_name("budi", "BUDI"));
        assert!(!same_name("budi", "andi"));
    }

    #[test]
    fn checks_length_in_chars() {
        assert!(require_text("name", "", 5).is_err());
        assert!(require_text("name", "ééééé", 5).is_ok());
        assert!(require_text("name", "éééééé", 5).is_err());
    }
}
