package com.smartagriculture;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.jdbc.core.JdbcTemplate;

@SpringBootTest
class DatabaseSchemaTests {

    @Autowired
    private JdbcTemplate jdbcTemplate;

    @Test
    void flywayCreatesSchemaAndSeedsRoles() {
        Integer roleCount = jdbcTemplate.queryForObject("select count(*) from roles", Integer.class);
        Integer tableCount = jdbcTemplate.queryForObject(
                "select count(*) from information_schema.tables "
                        + "where table_schema = database() and table_name in "
                        + "('users', 'farms', 'plots', 'devices', 'alert_rules', "
                        + "'alerts', 'device_commands', 'outbox_events')",
                Integer.class);

        assertThat(roleCount).isEqualTo(3);
        assertThat(tableCount).isEqualTo(8);
    }
}
